// Copyright 2026 soulteary
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command loadtest 是 portmap 的独立压测工具。
//
// 默认自包含模式下，工具内部启动一个 echo 目标服务，再用 forward.New(...)
// 在进程内起一个转发服务，然后由压测客户端对转发端口发压，形成
// client -> portmap -> echo 的完整链路，用于评估可靠性与吞吐量。
//
// 也支持 -external <addr>：跳过内建链路，直接压测外部已运行的 portmap 监听地址
// （此时需用户自备目标服务）。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/soulteary/portmap/internal/forward"
)

// options 保存命令行参数。
type options struct {
	proto        string
	conns        int
	duration     time.Duration
	requests     int
	payload      int
	mode         string
	external     string
	maxConns     int
	idleTimeout  time.Duration
	warmup       time.Duration
	format       string
	maxSamples   int
	maxErrorRate float64
	errorRateSet bool
	maxP95       time.Duration
}

func parseFlags(args []string) (*options, error) {
	fs := flag.NewFlagSet("loadtest", flag.ContinueOnError)
	o := &options{}
	fs.StringVar(&o.proto, "proto", "tcp", "转发协议：tcp 或 udp")
	fs.IntVar(&o.conns, "conns", 50, "并发连接/会话数")
	fs.DurationVar(&o.duration, "duration", 10*time.Second, "压测最长时长（所有模式的硬截止时间）")
	fs.IntVar(&o.requests, "requests", 0, "每连接成功请求数，0 表示仅按 duration 持续跑")
	fs.IntVar(&o.payload, "payload", 1024, "单次请求负载字节数")
	fs.StringVar(&o.mode, "mode", "throughput", "压测模式：throughput（长连接持续收发）或 connrate（短连接建立/关闭循环）")
	fs.StringVar(&o.external, "external", "", "外部 portmap 地址；为空则自建链路")
	fs.IntVar(&o.maxConns, "max-conns", 0, "内建转发服务的最大并发连接数，0 表示不限制")
	fs.DurationVar(&o.idleTimeout, "idle-timeout", 0, "内建转发服务的空闲超时，0 表示不启用")
	fs.DurationVar(&o.warmup, "warmup", time.Second, "预热时间，预热期数据不计入统计")
	fs.StringVar(&o.format, "format", "text", "输出格式：text 或 json")
	fs.IntVar(&o.maxSamples, "max-samples", defaultMaxLatencySamples, "最多保留的 RTT 样本数（使用蓄水池采样）")
	fs.Float64Var(&o.maxErrorRate, "max-error-rate", 0, "允许的最大错误率百分比，设置后超限退出 1")
	fs.DurationVar(&o.maxP95, "max-p95", 0, "允许的最大 p95 RTT，0 表示不设置阈值")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "max-error-rate" {
			o.errorRateSet = true
		}
	})
	if o.proto != "tcp" && o.proto != "udp" {
		return nil, fmt.Errorf("invalid -proto %q: must be tcp or udp", o.proto)
	}
	if o.mode != "throughput" && o.mode != "connrate" {
		return nil, fmt.Errorf("invalid -mode %q: must be throughput or connrate", o.mode)
	}
	if o.conns <= 0 {
		return nil, fmt.Errorf("invalid -conns %d: must be > 0", o.conns)
	}
	if o.payload <= 0 {
		return nil, fmt.Errorf("invalid -payload %d: must be > 0", o.payload)
	}
	if o.requests < 0 {
		return nil, fmt.Errorf("invalid -requests %d: must be >= 0", o.requests)
	}
	if o.duration <= 0 {
		return nil, fmt.Errorf("invalid -duration %s: must be > 0", o.duration)
	}
	if o.warmup < 0 {
		return nil, fmt.Errorf("invalid -warmup %s: must be >= 0", o.warmup)
	}
	if o.format != "text" && o.format != "json" {
		return nil, fmt.Errorf("invalid -format %q: must be text or json", o.format)
	}
	if o.maxSamples <= 0 {
		return nil, fmt.Errorf("invalid -max-samples %d: must be > 0", o.maxSamples)
	}
	if o.errorRateSet && (math.IsNaN(o.maxErrorRate) || math.IsInf(o.maxErrorRate, 0) || o.maxErrorRate < 0 || o.maxErrorRate > 100) {
		return nil, fmt.Errorf("invalid -max-error-rate %.4g: must be between 0 and 100", o.maxErrorRate)
	}
	if o.maxP95 < 0 {
		return nil, fmt.Errorf("invalid -max-p95 %s: must be >= 0", o.maxP95)
	}
	if o.maxConns < 0 {
		return nil, fmt.Errorf("invalid -max-conns %d: must be >= 0", o.maxConns)
	}
	if o.idleTimeout < 0 {
		return nil, fmt.Errorf("invalid -idle-timeout %s: must be >= 0", o.idleTimeout)
	}
	if o.proto == "udp" && o.payload > 65507 {
		return nil, fmt.Errorf("invalid -payload %d: UDP payload must be <= 65507", o.payload)
	}
	return o, nil
}

func main() {
	o, err := parseFlags(os.Args[1:])
	if err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "loadtest:", err)
		os.Exit(2)
	}
	if err := run(o); err != nil {
		fmt.Fprintln(os.Stderr, "loadtest:", err)
		os.Exit(1)
	}
}

// hostInfo 汇总当前主机与运行时环境信息。
type hostInfo struct {
	GOOS      string
	GOARCH    string
	NumCPU    int
	GoVersion string
	Hostname  string
	Mem       runtime.MemStats
}

func collectHostInfo() hostInfo {
	hi := hostInfo{
		GOOS:      runtime.GOOS,
		GOARCH:    runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
		GoVersion: runtime.Version(),
	}
	if name, err := os.Hostname(); err == nil {
		hi.Hostname = name
	} else {
		hi.Hostname = "unknown"
	}
	runtime.ReadMemStats(&hi.Mem)
	return hi
}

// freePort 返回一个当前空闲的本地 TCP 端口，用于为内建转发服务预留监听端口。
// forward.Server 不暴露其绑定的监听地址，因此需在构造 Config.Listen 前自行解析空闲端口
// （镜像 forward_test.go 中的 freePort：在 127.0.0.1:0 上 Listen，取端口后关闭）。
func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port, nil
}

// chain 描述一条自建压测链路（echo 目标 + 进程内转发服务）。
type chain struct {
	targetAddr string // 内建 echo 目标地址
	listenAddr string // 内建转发服务监听地址（压测客户端连接此地址）
	srv        *forward.Server
	closeEcho  func()
	cancel     context.CancelFunc
	done       chan error
}

// startEchoTarget 启动一个内建 echo 目标服务，返回其地址与关闭函数。
// TCP 用 io.Copy(c, c)；UDP 用 ReadFromUDP/WriteToUDP 回写。
func startEchoTarget(proto string) (addr string, closeFn func(), err error) {
	switch proto {
	case "tcp":
		ln, lerr := net.Listen("tcp", "127.0.0.1:0")
		if lerr != nil {
			return "", nil, lerr
		}
		go func() {
			for {
				c, aerr := ln.Accept()
				if aerr != nil {
					return
				}
				go func(c net.Conn) {
					defer func() { _ = c.Close() }()
					_, _ = io.Copy(c, c)
				}(c)
			}
		}()
		return ln.Addr().String(), func() { _ = ln.Close() }, nil
	case "udp":
		conn, cerr := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
		if cerr != nil {
			return "", nil, cerr
		}
		go func() {
			buf := make([]byte, 65536)
			for {
				n, from, rerr := conn.ReadFromUDP(buf)
				if rerr != nil {
					return
				}
				_, _ = conn.WriteToUDP(buf[:n], from)
			}
		}()
		return conn.LocalAddr().String(), func() { _ = conn.Close() }, nil
	default:
		return "", nil, fmt.Errorf("unsupported proto %q", proto)
	}
}

// startChain 起一个自包含链路：echo 目标 + 进程内转发服务。
func startChain(o *options) (*chain, error) {
	targetAddr, closeEcho, err := startEchoTarget(o.proto)
	if err != nil {
		return nil, fmt.Errorf("start echo target: %w", err)
	}

	port, err := freePort()
	if err != nil {
		closeEcho()
		return nil, fmt.Errorf("reserve free port: %w", err)
	}
	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)

	srv := forward.New(forward.Config{
		Listen:      listenAddr,
		Target:      targetAddr,
		Network:     o.proto,
		ReuseAddr:   true,
		MaxConns:    o.maxConns,
		IdleTimeout: o.idleTimeout,
		Logger:      log.New(io.Discard, "", 0),
		Quiet:       true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()

	if err := waitPortReady(listenAddr, o.proto); err != nil {
		cancel()
		closeEcho()
		return nil, err
	}

	return &chain{
		targetAddr: targetAddr,
		listenAddr: listenAddr,
		srv:        srv,
		closeEcho:  closeEcho,
		cancel:     cancel,
		done:       done,
	}, nil
}

// stop 优雅停止链路（取消转发服务并关闭 echo 目标），返回转发服务的退出错误。
func (c *chain) stop() error {
	c.cancel()
	var srvErr error
	select {
	case srvErr = <-c.done:
	case <-time.After(5 * time.Second):
		srvErr = fmt.Errorf("forward server did not exit within timeout")
	}
	c.closeEcho()
	return srvErr
}

// waitPortReady 等待监听地址就绪。TCP 通过 Dial 探测；UDP 监听立即可用。
func waitPortReady(addr, network string) error {
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if network == "udp" {
			time.Sleep(20 * time.Millisecond)
			return nil
		}
		c, err := net.Dial(network, addr)
		if err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("port %s not ready", addr)
}

// stats 保存全局聚合的计数器。每 worker 另用本地 slice 收集 RTT，结束后合并。
type stats struct {
	requests    atomic.Int64 // 成功请求数
	bytes       atomic.Int64 // 成功回环的字节数（发送即等量回显）
	newConns    atomic.Int64 // 新建连接/会话数
	errDial     atomic.Int64 // 建连失败
	errWrite    atomic.Int64 // 写入失败
	errRead     atomic.Int64 // 读取失败（含超时/丢包）
	errMismatch atomic.Int64 // 数据完整性校验失败
}

func (s *stats) totalErrors() int64 {
	return s.errDial.Load() + s.errWrite.Load() + s.errRead.Load() + s.errMismatch.Load()
}

// result 是一次压测运行的最终结果。
type result struct {
	elapsed   time.Duration
	requests  int64
	bytes     int64
	newConns  int64
	errors    int64
	errDial   int64
	errWrite  int64
	errRead   int64
	errMismat int64
	lat       []time.Duration // 已排序的 RTT
	latMin    time.Duration   // 全量成功请求的精确最小 RTT
	latMax    time.Duration   // 全量成功请求的精确最大 RTT
}

const defaultMaxLatencySamples = 100000

func latencySampleLimit(o *options) int {
	if o.maxSamples > 0 {
		return o.maxSamples
	}
	return defaultMaxLatencySamples
}

// run 执行一次完整压测。
func run(o *options) error {
	return runWithOutput(o, os.Stdout, os.Stderr)
}

func runWithOutput(o *options, stdout, stderr io.Writer) error {
	hi := collectHostInfo()

	// 建立链路（自建或外部）。
	var listenAddr string
	var ch *chain
	if o.external != "" {
		listenAddr = o.external
	} else {
		var err error
		ch, err = startChain(o)
		if err != nil {
			return err
		}
		listenAddr = ch.listenAddr
	}

	// 预热：短暂运行但不计入统计（此处以简单 sleep 让链路稳定，随后正式计时）。
	if o.warmup > 0 {
		warmCtx, warmCancel := context.WithTimeout(context.Background(), o.warmup)
		runWorkers(warmCtx, o, listenAddr, &stats{}, false)
		warmCancel()
	}

	// 正式压测。
	s := &stats{}
	ctx, cancel := context.WithTimeout(context.Background(), o.duration)
	defer cancel()

	start := time.Now()
	latency := runWorkers(ctx, o, listenAddr, s, true)
	elapsed := time.Since(start)
	cancel()

	lats := latency.samples
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })

	res := result{
		elapsed:   elapsed,
		requests:  s.requests.Load(),
		bytes:     s.bytes.Load(),
		newConns:  s.newConns.Load(),
		errors:    s.totalErrors(),
		errDial:   s.errDial.Load(),
		errWrite:  s.errWrite.Load(),
		errRead:   s.errRead.Load(),
		errMismat: s.errMismatch.Load(),
		lat:       lats,
		latMin:    latency.min,
		latMax:    latency.max,
	}

	// 校验 ActiveConns 归零（仅自建链路可观测）。
	// 转发服务在 ctx 取消后优雅退出：TCP 等待在途连接处理完毕，UDP 关闭所有会话并
	// 等待 relay 退出（active 在各自 defer 中归零）。因此可靠的归零校验应在
	// 优雅停止「之后」进行——UDP 会话默认要等 IdleTimeout(60s) 才自然回收，
	// 唯有停止服务才能确定性地驱动其归零。
	activeZero := true
	var activeAfter int64 = -1
	if ch != nil {
		if err := ch.stop(); err != nil {
			_, _ = fmt.Fprintln(stderr, "loadtest: forward server stop:", err)
		}
		activeAfter = ch.srv.ActiveConns()
		activeZero = activeAfter == 0
	}

	thresholdErr := checkThresholds(o, res)
	if o.format == "json" {
		if err := printJSONReport(stdout, o, hi, res, activeZero, activeAfter, ch != nil, thresholdErr == nil); err != nil {
			return err
		}
	} else {
		printReport(stdout, o, hi, res, activeZero, activeAfter, ch != nil)
	}
	return thresholdErr
}

// runWorkers 启动 o.conns 个 worker 并发发压，返回合并后的 RTT 切片。
// record 为 false 时（预热）不收集 RTT。
type latencyResult struct {
	samples []time.Duration
	min     time.Duration
	max     time.Duration
}

func runWorkers(ctx context.Context, o *options, addr string, s *stats, record bool) latencyResult {
	var wg sync.WaitGroup
	maxSamples := latencySampleLimit(o)
	sampler := newLatencySampler(maxSamples)
	for i := 0; i < o.conns; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w := &worker{
				o: o, addr: addr, s: s, record: record, sampler: sampler,
				sampleState: uint64(idx+1) * 0x9e3779b97f4a7c15,
			}
			if o.proto == "tcp" {
				w.runTCP(ctx)
			} else {
				w.runUDP(ctx)
			}
		}(i)
	}
	wg.Wait()

	minRTT, maxRTT := sampler.extrema()
	return latencyResult{samples: sampler.values(), min: minRTT, max: maxRTT}
}

// worker 表示单个并发压测协程。
type worker struct {
	o       *options
	addr    string
	s       *stats
	record  bool
	sampler *latencySampler
	// sampleState is private to this worker, so random admission never contends.
	sampleState uint64
}

type latencySample struct {
	sequence uint64
	value    time.Duration
}

// latencySampler uses lock-free reservoir admission over the combined RTT
// stream. Slots keep the highest sequence admitted to them, which preserves
// sequential reservoir replacement even when workers finish out of order.
type latencySampler struct {
	slots   []atomic.Pointer[latencySample]
	initial []latencySample
	seen    atomic.Uint64
	limit   uint64
	min     atomic.Int64
	max     atomic.Int64
}

func newLatencySampler(limit int) *latencySampler {
	sampler := &latencySampler{
		slots:   make([]atomic.Pointer[latencySample], limit),
		initial: make([]latencySample, limit),
		limit:   uint64(limit),
	}
	sampler.min.Store(1<<63 - 1)
	return sampler
}

func (s *latencySampler) add(v time.Duration, state *uint64) {
	updateAtomicMin(&s.min, int64(v))
	updateAtomicMax(&s.max, int64(v))
	sequence := s.seen.Add(1)
	index := sequence - 1
	if sequence > s.limit {
		index = nextSampleRandom(state) % sequence
		if index >= s.limit {
			return
		}
	}

	var candidate *latencySample
	if sequence <= s.limit {
		candidate = &s.initial[index]
		candidate.sequence = sequence
		candidate.value = v
	} else {
		candidate = &latencySample{sequence: sequence, value: v}
	}
	for {
		current := s.slots[index].Load()
		if current != nil && current.sequence > sequence {
			return
		}
		if s.slots[index].CompareAndSwap(current, candidate) {
			return
		}
	}
}

func updateAtomicMin(value *atomic.Int64, candidate int64) {
	for current := value.Load(); candidate < current; current = value.Load() {
		if value.CompareAndSwap(current, candidate) {
			return
		}
	}
}

func updateAtomicMax(value *atomic.Int64, candidate int64) {
	for current := value.Load(); candidate > current; current = value.Load() {
		if value.CompareAndSwap(current, candidate) {
			return
		}
	}
}

func (s *latencySampler) extrema() (time.Duration, time.Duration) {
	if s.seen.Load() == 0 {
		return 0, 0
	}
	return time.Duration(s.min.Load()), time.Duration(s.max.Load())
}

func (s *latencySampler) values() []time.Duration {
	values := make([]time.Duration, 0, min(int(s.seen.Load()), int(s.limit)))
	for i := range s.slots {
		if sample := s.slots[i].Load(); sample != nil {
			values = append(values, sample.value)
		}
	}
	return values
}

func nextSampleRandom(state *uint64) uint64 {
	if *state == 0 {
		*state = 1
	}
	*state ^= *state << 13
	*state ^= *state >> 7
	*state ^= *state << 17
	return *state
}

// makePayload 构造校验用的确定性负载。
func makePayload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte('a' + (i % 26))
	}
	return b
}

// budget 返回本 worker 应执行的请求次数上限；requests<=0 时返回 -1 表示按 ctx 持续跑。
func (w *worker) budget() int {
	if w.o.requests > 0 {
		return w.o.requests
	}
	return -1
}

// ctxDone 判断 ctx 是否已取消。
func ctxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

const retryBackoff = 10 * time.Millisecond

// waitBeforeRetry 为连续失败提供短暂退避，同时确保全局截止时间能立即打断等待。
func waitBeforeRetry(ctx context.Context) bool {
	timer := time.NewTimer(retryBackoff)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// operationDeadline 返回单次 I/O 上限与全局压测截止时间中较早的一个。
func operationDeadline(ctx context.Context, limit time.Duration) time.Time {
	deadline := time.Now().Add(limit)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		return ctxDeadline
	}
	return deadline
}

// runTCP 执行 TCP 压测：throughput 复用同一连接循环收发；connrate 每次新建/关闭连接。
func (w *worker) runTCP(ctx context.Context) {
	payload := makePayload(w.o.payload)
	buf := make([]byte, w.o.payload)
	budget := w.budget()
	done := 0
	dialer := net.Dialer{Timeout: 5 * time.Second}

	if w.o.mode == "throughput" {
		conn, err := dialer.DialContext(ctx, "tcp", w.addr)
		if err != nil {
			w.s.errDial.Add(1)
			return
		}
		w.s.newConns.Add(1)
		defer func() { _ = conn.Close() }()
		for !ctxDone(ctx) {
			if budget >= 0 && done >= budget {
				break
			}
			rtt, ok := tcpRoundTrip(ctx, conn, payload, buf, w.s)
			if !ok {
				return
			}
			if w.record {
				w.sampler.add(rtt, &w.sampleState)
			}
			done++
		}
		return
	}

	// connrate：每次请求新建连接、收发一轮、关闭。
	for !ctxDone(ctx) {
		if budget >= 0 && done >= budget {
			break
		}
		start := time.Now()
		conn, err := dialer.DialContext(ctx, "tcp", w.addr)
		if err != nil {
			// 本次拨号已在截止时间前开始；即使返回时全局上下文刚好
			// 到期，也必须记录该次失败，避免报告出现 0 请求、0 错误。
			w.s.errDial.Add(1)
			if !waitBeforeRetry(ctx) {
				break
			}
			continue
		}
		w.s.newConns.Add(1)
		_, ok := tcpRoundTrip(ctx, conn, payload, buf, w.s)
		_ = conn.Close()
		if !ok {
			if !waitBeforeRetry(ctx) {
				break
			}
			continue
		}
		if w.record {
			w.sampler.add(time.Since(start), &w.sampleState)
		}
		done++
	}
}

// tcpRoundTrip 写入 payload 并 ReadFull 读回等量字节，校验数据完整性。
// 返回本次 RTT 与是否成功。
func tcpRoundTrip(ctx context.Context, conn net.Conn, payload, buf []byte, s *stats) (time.Duration, bool) {
	_ = conn.SetDeadline(operationDeadline(ctx, 10*time.Second))
	start := time.Now()
	if _, err := conn.Write(payload); err != nil {
		// 一旦 RoundTrip 已开始，其结果就属于本次压测统计；全局截止
		// 时间只阻止新请求，不应吞掉正在执行的失败。
		s.errWrite.Add(1)
		return 0, false
	}
	if _, err := io.ReadFull(conn, buf); err != nil {
		s.errRead.Add(1)
		return 0, false
	}
	rtt := time.Since(start)
	if !bytesEqual(buf, payload) {
		s.errMismatch.Add(1)
		return 0, false
	}
	s.requests.Add(1)
	s.bytes.Add(int64(len(payload)))
	return rtt, true
}

// runUDP 执行 UDP 压测：发送数据报并按超时读回，校验回显内容；丢包/超时按错误统计。
func (w *worker) runUDP(ctx context.Context) {
	payload := makePayload(w.o.payload)
	buf := make([]byte, w.o.payload+64)
	budget := w.budget()
	done := 0

	dialer := net.Dialer{Timeout: 5 * time.Second}
	dial := func() *net.UDPConn {
		raw, derr := dialer.DialContext(ctx, "udp", w.addr)
		if derr != nil {
			w.s.errDial.Add(1)
			return nil
		}
		c, ok := raw.(*net.UDPConn)
		if !ok {
			_ = raw.Close()
			w.s.errDial.Add(1)
			return nil
		}
		w.s.newConns.Add(1)
		return c
	}

	var conn *net.UDPConn
	if w.o.mode == "throughput" {
		conn = dial()
		if conn == nil {
			return
		}
		defer func() { _ = conn.Close() }()
	}

	for !ctxDone(ctx) {
		if budget >= 0 && done >= budget {
			break
		}
		c := conn
		if w.o.mode == "connrate" {
			c = dial()
			if c == nil {
				if !waitBeforeRetry(ctx) {
					break
				}
				continue
			}
		}
		rtt, ok := udpRoundTrip(ctx, c, payload, buf, w.s)
		if w.o.mode == "connrate" {
			_ = c.Close()
		}
		if ok {
			if w.record {
				w.sampler.add(rtt, &w.sampleState)
			}
			done++
		} else if !waitBeforeRetry(ctx) {
			break
		}
	}
}

// udpRoundTrip 发送一个数据报并按超时读回，校验回显内容。
func udpRoundTrip(ctx context.Context, conn *net.UDPConn, payload, buf []byte, s *stats) (time.Duration, bool) {
	_ = conn.SetDeadline(operationDeadline(ctx, 2*time.Second))
	start := time.Now()
	if _, err := conn.Write(payload); err != nil {
		s.errWrite.Add(1)
		return 0, false
	}
	n, err := conn.Read(buf)
	if err != nil {
		// UDP 丢包/超时：按读取错误统计而非致命。
		s.errRead.Add(1)
		return 0, false
	}
	rtt := time.Since(start)
	if n != len(payload) || !bytesEqual(buf[:n], payload) {
		s.errMismatch.Add(1)
		return 0, false
	}
	s.requests.Add(1)
	s.bytes.Add(int64(len(payload)))
	return rtt, true
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// percentile 返回已排序切片的分位数（p 取 0..100）。空切片返回 0。
func percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 100 {
		return sorted[n-1]
	}
	idx := int((p / 100) * float64(n))
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

func resultErrorRate(res result) float64 {
	total := res.requests + res.errors
	if total == 0 {
		return 0
	}
	return float64(res.errors) / float64(total) * 100
}

func checkThresholds(o *options, res result) error {
	var failures []string
	if o.errorRateSet {
		if rate := resultErrorRate(res); rate > o.maxErrorRate {
			failures = append(failures, fmt.Sprintf("error rate %.4f%% exceeds %.4f%%", rate, o.maxErrorRate))
		}
	}
	if o.maxP95 > 0 {
		if len(res.lat) == 0 {
			failures = append(failures, "p95 threshold cannot be evaluated without successful samples")
		} else if p95 := percentile(res.lat, 95); p95 > o.maxP95 {
			failures = append(failures, fmt.Sprintf("p95 %s exceeds %s", p95, o.maxP95))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("threshold failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

type jsonReport struct {
	Host struct {
		Hostname  string `json:"hostname"`
		OS        string `json:"os"`
		Arch      string `json:"arch"`
		CPUs      int    `json:"cpus"`
		GoVersion string `json:"go_version"`
	} `json:"host"`
	Config struct {
		Chain      string `json:"chain"`
		Target     string `json:"target"`
		Protocol   string `json:"protocol"`
		Mode       string `json:"mode"`
		Conns      int    `json:"conns"`
		Payload    int    `json:"payload_bytes"`
		MaxSamples int    `json:"max_latency_samples"`
	} `json:"config"`
	Results struct {
		ElapsedMs   float64 `json:"elapsed_ms"`
		Requests    int64   `json:"requests"`
		NewConns    int64   `json:"new_connections"`
		Bytes       int64   `json:"bytes"`
		RequestsPS  float64 `json:"requests_per_second"`
		Throughput  float64 `json:"throughput_bytes_per_second"`
		SampleCount int     `json:"latency_sample_count"`
		MinUs       int64   `json:"min_us"`
		P50Us       int64   `json:"p50_us"`
		P95Us       int64   `json:"p95_us"`
		P99Us       int64   `json:"p99_us"`
		MaxUs       int64   `json:"max_us"`
	} `json:"results"`
	Reliability struct {
		Errors               int64   `json:"errors"`
		ErrorRate            float64 `json:"error_rate_percent"`
		Dial                 int64   `json:"dial_errors"`
		Write                int64   `json:"write_errors"`
		Read                 int64   `json:"read_errors"`
		Mismatch             int64   `json:"mismatch_errors"`
		ActiveAfter          *int64  `json:"active_connections_after,omitempty"`
		ActiveReturnedToZero *bool   `json:"active_connections_returned_to_zero,omitempty"`
	} `json:"reliability"`
	ThresholdsPassed bool `json:"thresholds_passed"`
}

func printJSONReport(w io.Writer, o *options, hi hostInfo, res result, activeZero bool, activeAfter int64, builtin, passed bool) error {
	report := jsonReport{ThresholdsPassed: passed}
	report.Host.Hostname = hi.Hostname
	report.Host.OS = hi.GOOS
	report.Host.Arch = hi.GOARCH
	report.Host.CPUs = hi.NumCPU
	report.Host.GoVersion = hi.GoVersion
	report.Config.Chain = "external"
	report.Config.Target = o.external
	if builtin {
		report.Config.Chain = "built-in"
		report.Config.Target = "built-in echo"
	}
	report.Config.Protocol = o.proto
	report.Config.Mode = o.mode
	report.Config.Conns = o.conns
	report.Config.Payload = o.payload
	report.Config.MaxSamples = latencySampleLimit(o)

	sec := res.elapsed.Seconds()
	if sec <= 0 {
		sec = 1e-9
	}
	report.Results.ElapsedMs = float64(res.elapsed) / float64(time.Millisecond)
	report.Results.Requests = res.requests
	report.Results.NewConns = res.newConns
	report.Results.Bytes = res.bytes
	report.Results.RequestsPS = float64(res.requests) / sec
	report.Results.Throughput = float64(res.bytes) / sec
	report.Results.SampleCount = len(res.lat)
	report.Results.MinUs = res.latMin.Microseconds()
	report.Results.P50Us = percentile(res.lat, 50).Microseconds()
	report.Results.P95Us = percentile(res.lat, 95).Microseconds()
	report.Results.P99Us = percentile(res.lat, 99).Microseconds()
	report.Results.MaxUs = res.latMax.Microseconds()

	report.Reliability.Errors = res.errors
	report.Reliability.ErrorRate = resultErrorRate(res)
	report.Reliability.Dial = res.errDial
	report.Reliability.Write = res.errWrite
	report.Reliability.Read = res.errRead
	report.Reliability.Mismatch = res.errMismat
	if builtin {
		report.Reliability.ActiveAfter = &activeAfter
		report.Reliability.ActiveReturnedToZero = &activeZero
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

// printReport 打印文本报告：主机环境块 + 配置块 + 结果块。
func printReport(w io.Writer, o *options, hi hostInfo, res result, activeZero bool, activeAfter int64, builtin bool) {
	sec := res.elapsed.Seconds()
	if sec <= 0 {
		sec = 1e-9
	}
	mbps := float64(res.bytes) / (1024 * 1024) / sec
	gbps := float64(res.bytes) * 8 / 1e9 / sec
	reqps := float64(res.requests) / sec
	connps := float64(res.newConns) / sec

	errRate := resultErrorRate(res)

	// p / pf 将报告写入 w，忽略写标准输出时不可恢复的错误。
	p := func(s string) { _, _ = fmt.Fprintln(w, s) }
	pf := func(format string, a ...any) { _, _ = fmt.Fprintf(w, format, a...) }

	p("==================== portmap loadtest report ====================")
	p("[ Host / Runtime ]")
	pf("  hostname     : %s\n", hi.Hostname)
	pf("  os/arch      : %s/%s\n", hi.GOOS, hi.GOARCH)
	pf("  num cpu      : %d\n", hi.NumCPU)
	pf("  go version   : %s\n", hi.GoVersion)
	pf("  mem alloc    : %.2f MiB (sys %.2f MiB)\n",
		float64(hi.Mem.Alloc)/(1024*1024), float64(hi.Mem.Sys)/(1024*1024))

	p("[ Config ]")
	mode := "external"
	target := o.external
	if builtin {
		mode = "built-in chain (echo <- portmap <- client)"
		target = "(built-in echo)"
	}
	pf("  chain        : %s\n", mode)
	pf("  target       : %s\n", target)
	pf("  proto        : %s\n", o.proto)
	pf("  test mode    : %s\n", o.mode)
	pf("  conns        : %d\n", o.conns)
	if o.requests > 0 {
		pf("  requests     : %d successful per conn\n", o.requests)
	}
	pf("  max duration : %s\n", o.duration)
	pf("  payload      : %d bytes\n", o.payload)
	pf("  warmup       : %s\n", o.warmup)
	pf("  max-conns    : %d\n", o.maxConns)
	pf("  idle-timeout : %s\n", o.idleTimeout)

	p("[ Results ]")
	pf("  elapsed      : %s\n", res.elapsed.Round(time.Millisecond))
	pf("  requests ok  : %d\n", res.requests)
	pf("  new conns    : %d\n", res.newConns)
	pf("  bytes echoed : %d (%.2f MiB)\n", res.bytes, float64(res.bytes)/(1024*1024))
	pf("  throughput   : %.2f MB/s | %.3f Gbps\n", mbps, gbps)
	pf("  req/s        : %.2f\n", reqps)
	if o.mode == "connrate" {
		pf("  conns/s      : %.2f\n", connps)
	}

	p("[ Latency (RTT) ]")
	if len(res.lat) == 0 {
		p("  (no successful samples)")
	} else {
		pf("  min          : %s\n", res.latMin.Round(time.Microsecond))
		pf("  p50          : %s\n", percentile(res.lat, 50).Round(time.Microsecond))
		pf("  p95          : %s\n", percentile(res.lat, 95).Round(time.Microsecond))
		pf("  p99          : %s\n", percentile(res.lat, 99).Round(time.Microsecond))
		pf("  max          : %s\n", res.latMax.Round(time.Microsecond))
	}
	pf("  samples      : %d/%d successful RTTs retained\n", len(res.lat), res.requests)

	p("[ Reliability ]")
	pf("  errors       : %d (rate %.4f%%)\n", res.errors, errRate)
	pf("    dial       : %d\n", res.errDial)
	pf("    write      : %d\n", res.errWrite)
	pf("    read/timeout: %d\n", res.errRead)
	pf("    mismatch   : %d\n", res.errMismat)
	if builtin {
		if activeZero {
			p("  active conns : 0 (returned to zero) OK")
		} else {
			pf("  active conns : %d (DID NOT return to zero) FAIL\n", activeAfter)
		}
	} else {
		p("  active conns : n/a (external chain)")
	}
	p("=================================================================")
}
