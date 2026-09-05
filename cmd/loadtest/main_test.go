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

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseFlagsValidatesTerminationAndLimits(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "zero duration", args: []string{"-duration", "0s"}},
		{name: "negative duration", args: []string{"-duration", "-1s"}},
		{name: "negative warmup", args: []string{"-warmup", "-1s"}},
		{name: "negative max conns", args: []string{"-max-conns", "-1"}},
		{name: "negative idle timeout", args: []string{"-idle-timeout", "-1s"}},
		{name: "oversized udp datagram", args: []string{"-proto", "udp", "-payload", "65508"}},
		{name: "bad output format", args: []string{"-format", "xml"}},
		{name: "zero samples", args: []string{"-max-samples", "0"}},
		{name: "negative p95", args: []string{"-max-p95", "-1ms"}},
		{name: "error rate over 100", args: []string{"-max-error-rate", "101"}},
		{name: "NaN error rate", args: []string{"-max-error-rate", "NaN"}},
		{name: "infinite error rate", args: []string{"-max-error-rate", "+Inf"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseFlags(tc.args); err == nil {
				t.Fatalf("parseFlags(%v) 应返回错误", tc.args)
			}
		})
	}
}

func TestFixedRequestModeStopsAtDurationWhenTargetFails(t *testing.T) {
	tests := []struct {
		name  string
		proto string
		mode  string
		addr  func(*testing.T) string
	}{
		{
			name:  "tcp connrate",
			proto: "tcp",
			mode:  "connrate",
			addr: func(t *testing.T) string {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				addr := ln.Addr().String()
				_ = ln.Close()
				return addr
			},
		},
		{
			name:  "udp throughput",
			proto: "udp",
			mode:  "throughput",
			addr: func(t *testing.T) string {
				conn, err := net.ListenPacket("udp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				addr := conn.LocalAddr().String()
				_ = conn.Close()
				return addr
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := &options{
				proto:    tc.proto,
				mode:     tc.mode,
				conns:    1,
				duration: 80 * time.Millisecond,
				requests: 1,
				payload:  32,
				external: tc.addr(t),
			}
			started := time.Now()
			if err := runWithOutput(o, io.Discard, io.Discard); err != nil {
				t.Fatalf("runWithOutput 返回错误: %v", err)
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("固定请求模式在目标失败时未按 duration 结束: %s", elapsed)
			}
		})
	}
}

func TestRoundTripDeadlineFailuresAreRecorded(t *testing.T) {
	t.Run("tcp", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer func() { _ = serverConn.Close() }()
		defer func() { _ = clientConn.Close() }()
		payload := makePayload(32)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		received := make(chan struct{})
		go func() {
			buf := make([]byte, len(payload))
			_, _ = io.ReadFull(serverConn, buf)
			close(received)
			<-ctx.Done()
		}()

		s := &stats{}
		if _, ok := tcpRoundTrip(ctx, clientConn, payload, make([]byte, len(payload)), s); ok {
			t.Fatal("静默 TCP 目标不应返回成功")
		}
		if got := s.errRead.Load(); got != 1 {
			t.Fatalf("截止时间触发的 TCP 读取失败数=%d，期望 1", got)
		}
		select {
		case <-received:
		case <-time.After(time.Second):
			t.Fatal("TCP 请求未实际发送到目标")
		}
	})

	t.Run("udp", func(t *testing.T) {
		serverConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
		if err != nil {
			t.Fatalf("监听 UDP 失败: %v", err)
		}
		defer func() { _ = serverConn.Close() }()
		clientConn, err := net.DialUDP("udp", nil, serverConn.LocalAddr().(*net.UDPAddr))
		if err != nil {
			t.Fatalf("连接 UDP 目标失败: %v", err)
		}
		defer func() { _ = clientConn.Close() }()
		payload := makePayload(32)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		received := make(chan struct{})
		go func() {
			buf := make([]byte, len(payload))
			_, _, _ = serverConn.ReadFromUDP(buf)
			close(received)
			<-ctx.Done()
		}()

		s := &stats{}
		if _, ok := udpRoundTrip(ctx, clientConn, payload, make([]byte, len(payload)+64), s); ok {
			t.Fatal("静默 UDP 目标不应返回成功")
		}
		if got := s.errRead.Load(); got != 1 {
			t.Fatalf("截止时间触发的 UDP 读取失败数=%d，期望 1", got)
		}
		select {
		case <-received:
		case <-time.After(time.Second):
			t.Fatal("UDP 请求未实际发送到目标")
		}
	})
}

// TestParseFlagsDefaults 验证默认值解析正确。
func TestParseFlagsDefaults(t *testing.T) {
	o, err := parseFlags(nil)
	if err != nil {
		t.Fatalf("parseFlags(nil): %v", err)
	}
	if o.proto != "tcp" || o.conns != 50 || o.duration != 10*time.Second ||
		o.payload != 1024 || o.mode != "throughput" || o.warmup != time.Second ||
		o.format != "text" || o.maxSamples != defaultMaxLatencySamples || o.errorRateSet {
		t.Fatalf("默认值不符: %+v", o)
	}
}

func TestLatencySamplerIsBounded(t *testing.T) {
	sampler := newLatencySampler(10, 1)
	for i := 1; i <= 10000; i++ {
		sampler.add(time.Duration(i))
	}
	if sampler.seen != 10000 {
		t.Fatalf("seen=%d，期望 10000", sampler.seen)
	}
	if len(sampler.samples) != 10 {
		t.Fatalf("samples=%d，期望 10", len(sampler.samples))
	}
}

func TestLatencySamplerUsesGlobalConcurrentLimit(t *testing.T) {
	sampler := newLatencySampler(7, 1)
	var wg sync.WaitGroup
	for worker := 0; worker < 20; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				sampler.add(time.Duration(worker*1000 + i))
			}
		}(worker)
	}
	wg.Wait()
	if sampler.seen != 20000 {
		t.Fatalf("seen=%d, want 20000", sampler.seen)
	}
	if len(sampler.samples) != 7 {
		t.Fatalf("samples=%d, want global limit 7", len(sampler.samples))
	}
}

func TestThresholds(t *testing.T) {
	res := result{
		requests: 90,
		errors:   10,
		lat:      []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond},
	}
	if err := checkThresholds(&options{errorRateSet: true, maxErrorRate: 10, maxP95: 3 * time.Millisecond}, res); err != nil {
		t.Fatalf("边界值应通过: %v", err)
	}
	if err := checkThresholds(&options{errorRateSet: true, maxErrorRate: 9}, res); err == nil {
		t.Fatal("错误率超限应失败")
	}
	if err := checkThresholds(&options{maxP95: 2 * time.Millisecond}, res); err == nil {
		t.Fatal("p95 超限应失败")
	}
	if err := checkThresholds(&options{maxP95: time.Second}, result{}); err == nil {
		t.Fatal("无成功样本时不能通过 p95 阈值")
	}
}

func TestJSONReport(t *testing.T) {
	o := &options{proto: "tcp", mode: "throughput", conns: 2, payload: 64, maxSamples: 7}
	res := result{
		elapsed:  time.Second,
		requests: 3,
		bytes:    192,
		newConns: 2,
		lat:      []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond},
	}
	var buf bytes.Buffer
	if err := printJSONReport(&buf, o, collectHostInfo(), res, true, 0, true, true); err != nil {
		t.Fatalf("printJSONReport: %v", err)
	}
	var decoded struct {
		Results struct {
			Requests    int64 `json:"requests"`
			SampleCount int   `json:"latency_sample_count"`
		} `json:"results"`
		ThresholdsPassed bool `json:"thresholds_passed"`
	}
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON 无法解析: %v\n%s", err, buf.String())
	}
	if decoded.Results.Requests != 3 || decoded.Results.SampleCount != 3 || !decoded.ThresholdsPassed {
		t.Fatalf("JSON 内容不符: %+v", decoded)
	}
}

// TestParseFlagsRejectsBadEnums 覆盖非法 proto / mode / conns / payload / requests。
func TestParseFlagsRejectsBadEnums(t *testing.T) {
	cases := [][]string{
		{"-proto", "sctp"},
		{"-mode", "bogus"},
		{"-conns", "0"},
		{"-payload", "0"},
		{"-requests", "-1"},
	}
	for _, args := range cases {
		if _, err := parseFlags(args); err == nil {
			t.Errorf("parseFlags(%v) 应返回错误", args)
		}
	}
}

// TestBytesEqual 覆盖相等、长度不同、内容不同三种情况。
func TestBytesEqual(t *testing.T) {
	if !bytesEqual([]byte("abc"), []byte("abc")) {
		t.Error("相同内容应相等")
	}
	if bytesEqual([]byte("abc"), []byte("ab")) {
		t.Error("长度不同不应相等")
	}
	if bytesEqual([]byte("abc"), []byte("abd")) {
		t.Error("内容不同不应相等")
	}
	if !bytesEqual(nil, []byte{}) {
		t.Error("空切片应相等")
	}
}

// TestPercentile 覆盖空切片、边界（p<=0、p>=100）与中间分位。
func TestPercentile(t *testing.T) {
	if got := percentile(nil, 50); got != 0 {
		t.Errorf("空切片应返回 0，实际 %s", got)
	}
	sorted := []time.Duration{10, 20, 30, 40, 50}
	if got := percentile(sorted, 0); got != 10 {
		t.Errorf("p0=%s，期望 10ns", got)
	}
	if got := percentile(sorted, 100); got != 50 {
		t.Errorf("p100=%s，期望 50ns", got)
	}
	if got := percentile(sorted, 50); got != 30 {
		t.Errorf("p50=%s，期望 30ns", got)
	}
	// p 非常接近 100 时索引不应越界。
	if got := percentile(sorted, 99.999); got != 50 {
		t.Errorf("p99.999=%s，期望 50ns", got)
	}
}

// TestMakePayload 验证负载确定性生成（长度与循环字节）。
func TestMakePayload(t *testing.T) {
	p := makePayload(30)
	if len(p) != 30 {
		t.Fatalf("长度=%d，期望 30", len(p))
	}
	if p[0] != 'a' || p[25] != 'z' || p[26] != 'a' {
		t.Fatalf("负载循环字节错误: %q", p[:27])
	}
	if len(makePayload(0)) != 0 {
		t.Error("长度 0 应返回空切片")
	}
}

// TestFreePort 验证 freePort 返回可用端口（可再次监听）。
func TestFreePort(t *testing.T) {
	port, err := freePort()
	if err != nil {
		t.Fatalf("freePort: %v", err)
	}
	if port <= 0 || port > 65535 {
		t.Fatalf("端口越界: %d", port)
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("返回的端口不可用: %v", err)
	}
	_ = ln.Close()
}

// TestCollectHostInfo 验证主机信息采集非空。
func TestCollectHostInfo(t *testing.T) {
	hi := collectHostInfo()
	if hi.GOOS == "" || hi.GOARCH == "" || hi.NumCPU <= 0 || hi.GoVersion == "" || hi.Hostname == "" {
		t.Fatalf("主机信息字段缺失: %+v", hi)
	}
}

// TestBudget 验证 budget：requests>0 时返回该值，否则返回 -1。
func TestBudget(t *testing.T) {
	w := &worker{o: &options{requests: 5}}
	if got := w.budget(); got != 5 {
		t.Errorf("budget=%d，期望 5", got)
	}
	w = &worker{o: &options{requests: 0}}
	if got := w.budget(); got != -1 {
		t.Errorf("budget=%d，期望 -1", got)
	}
}

// TestCtxDone 覆盖 ctxDone 的已取消与未取消。
func TestCtxDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	if ctxDone(ctx) {
		t.Error("未取消的 ctx 不应为 done")
	}
	cancel()
	if !ctxDone(ctx) {
		t.Error("已取消的 ctx 应为 done")
	}
}

// TestWaitBeforeRetry 覆盖正常退避与 ctx 取消打断两条路径。
func TestWaitBeforeRetry(t *testing.T) {
	if !waitBeforeRetry(context.Background()) {
		t.Error("正常退避应返回 true")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitBeforeRetry(ctx) {
		t.Error("ctx 已取消时应返回 false")
	}
}

// TestOperationDeadline 验证返回单次上限与全局截止时间中较早者。
func TestOperationDeadline(t *testing.T) {
	// 无 ctx 截止时间时返回 now+limit。
	got := operationDeadline(context.Background(), time.Hour)
	if time.Until(got) < 30*time.Minute {
		t.Fatalf("无 ctx deadline 时应约为 now+limit，实际剩余 %s", time.Until(got))
	}
	// ctx 截止时间更早时返回 ctx deadline。
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	got = operationDeadline(ctx, time.Hour)
	if time.Until(got) > time.Minute {
		t.Fatalf("ctx deadline 更早时应返回 ctx deadline，实际剩余 %s", time.Until(got))
	}
}

// TestStartEchoTarget 覆盖 TCP/UDP echo 目标与不支持协议。
func TestStartEchoTarget(t *testing.T) {
	t.Run("tcp", func(t *testing.T) {
		addr, closeFn, err := startEchoTarget("tcp")
		if err != nil {
			t.Fatalf("startEchoTarget(tcp): %v", err)
		}
		defer closeFn()
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("拨号 echo 失败: %v", err)
		}
		defer func() { _ = conn.Close() }()
		if _, err := conn.Write([]byte("ping")); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		buf := make([]byte, 4)
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := io.ReadFull(conn, buf); err != nil {
			t.Fatalf("读回失败: %v", err)
		}
		if !bytes.Equal(buf, []byte("ping")) {
			t.Fatalf("echo=%q", buf)
		}
	})
	t.Run("udp", func(t *testing.T) {
		addr, closeFn, err := startEchoTarget("udp")
		if err != nil {
			t.Fatalf("startEchoTarget(udp): %v", err)
		}
		defer closeFn()
		conn, err := net.Dial("udp", addr)
		if err != nil {
			t.Fatalf("拨号 UDP echo 失败: %v", err)
		}
		defer func() { _ = conn.Close() }()
		if _, err := conn.Write([]byte("pong")); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		buf := make([]byte, 8)
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("读回失败: %v", err)
		}
		if !bytes.Equal(buf[:n], []byte("pong")) {
			t.Fatalf("echo=%q", buf[:n])
		}
	})
	t.Run("不支持协议", func(t *testing.T) {
		if _, _, err := startEchoTarget("sctp"); err == nil {
			t.Fatal("不支持的协议应返回错误")
		}
	})
}

// TestWaitPortReady 覆盖 TCP 就绪、UDP 立即就绪、以及未就绪超时三条路径。
func TestWaitPortReady(t *testing.T) {
	t.Run("tcp 就绪", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = ln.Close() }()
		go func() {
			for {
				c, aerr := ln.Accept()
				if aerr != nil {
					return
				}
				_ = c.Close()
			}
		}()
		if err := waitPortReady(ln.Addr().String(), "tcp"); err != nil {
			t.Fatalf("已就绪端口应返回 nil: %v", err)
		}
	})
	t.Run("udp 立即就绪", func(t *testing.T) {
		if err := waitPortReady("127.0.0.1:1", "udp"); err != nil {
			t.Fatalf("UDP 应立即就绪: %v", err)
		}
	})
	t.Run("tcp 未就绪超时", func(t *testing.T) {
		// 预留一个空闲端口但不监听，使 Dial 持续失败直到超时。
		ln, _ := net.Listen("tcp", "127.0.0.1:0")
		addr := ln.Addr().String()
		_ = ln.Close()
		start := time.Now()
		if err := waitPortReady(addr, "tcp"); err == nil {
			t.Fatal("未监听端口应超时返回错误")
		}
		if time.Since(start) < 2*time.Second {
			t.Fatalf("超时过早返回: %s", time.Since(start))
		}
	})
}

// TestPrintReportBuiltinAndExternal 验证报告在内建链路与外部链路两种模式下
// 输出关键行，且分位数区块随样本存在/缺失切换。
func TestPrintReport(t *testing.T) {
	o := &options{proto: "tcp", mode: "connrate", conns: 10, requests: 3, duration: time.Second, payload: 64, warmup: 0}
	hi := collectHostInfo()

	t.Run("内建链路含样本", func(t *testing.T) {
		res := result{
			elapsed:  time.Second,
			requests: 100,
			bytes:    6400,
			newConns: 100,
			lat:      []time.Duration{time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond},
		}
		var buf bytes.Buffer
		printReport(&buf, o, hi, res, true, 0, true)
		out := buf.String()
		for _, want := range []string{"portmap loadtest report", "built-in chain", "requests ok  : 100", "conns/s", "p50", "active conns : 0 (returned to zero) OK"} {
			if !strings.Contains(out, want) {
				t.Errorf("报告缺少 %q", want)
			}
		}
	})

	t.Run("外部链路无样本", func(t *testing.T) {
		ext := &options{proto: "tcp", mode: "throughput", conns: 1, duration: time.Second, payload: 64, external: "1.2.3.4:9"}
		res := result{elapsed: time.Second, errors: 5, errDial: 5}
		var buf bytes.Buffer
		printReport(&buf, ext, hi, res, true, 3, false)
		out := buf.String()
		for _, want := range []string{"external", "(no successful samples)", "active conns : n/a (external chain)"} {
			if !strings.Contains(out, want) {
				t.Errorf("报告缺少 %q", want)
			}
		}
	})

	t.Run("内建链路 active 未归零", func(t *testing.T) {
		res := result{elapsed: time.Second, requests: 1}
		var buf bytes.Buffer
		printReport(&buf, o, hi, res, false, 7, true)
		if !strings.Contains(buf.String(), "DID NOT return to zero") {
			t.Error("active 未归零应输出 FAIL 提示")
		}
	})
}

// TestRunWithOutputBuiltinChain 端到端跑一次内建链路压测（TCP throughput，
// 极短时长），覆盖 startChain/stop/runWorkers/run 主路径。
func TestRunWithOutputBuiltinChain(t *testing.T) {
	o := &options{
		proto:    "tcp",
		mode:     "throughput",
		conns:    2,
		duration: 120 * time.Millisecond,
		requests: 0,
		payload:  64,
		warmup:   20 * time.Millisecond,
	}
	var out bytes.Buffer
	if err := runWithOutput(o, &out, io.Discard); err != nil {
		t.Fatalf("runWithOutput: %v", err)
	}
	report := out.String()
	if !strings.Contains(report, "built-in chain") {
		t.Fatalf("报告未标记内建链路:\n%s", report)
	}
	if !strings.Contains(report, "requests ok") {
		t.Fatalf("报告缺少请求统计:\n%s", report)
	}
}

// TestRunWithOutputAllProtoModes 覆盖内建链路下 tcp/udp × throughput/connrate
// 四种组合的成功收发路径（startChain 的 UDP 分支、runUDP/udpRoundTrip 成功、
// runTCP connrate 成功），并断言成功请求数为正、无数据完整性错误。
func TestRunWithOutputAllProtoModes(t *testing.T) {
	combos := []struct {
		proto string
		mode  string
	}{
		{"tcp", "throughput"},
		{"tcp", "connrate"},
		{"udp", "throughput"},
		{"udp", "connrate"},
	}
	for _, c := range combos {
		t.Run(c.proto+"_"+c.mode, func(t *testing.T) {
			o := &options{
				proto:    c.proto,
				mode:     c.mode,
				conns:    2,
				duration: 150 * time.Millisecond,
				requests: 0,
				payload:  128,
				warmup:   0,
			}
			var out bytes.Buffer
			if err := runWithOutput(o, &out, io.Discard); err != nil {
				t.Fatalf("runWithOutput(%s/%s): %v", c.proto, c.mode, err)
			}
			report := out.String()
			if !strings.Contains(report, "built-in chain") {
				t.Fatalf("报告未标记内建链路:\n%s", report)
			}
			// 内建 echo 链路应有成功请求且不出现数据完整性错误。
			if !strings.Contains(report, "mismatch   : 0") {
				t.Fatalf("内建链路不应出现数据完整性错误:\n%s", report)
			}
			if strings.Contains(report, "requests ok  : 0\n") {
				t.Fatalf("内建链路应有成功请求:\n%s", report)
			}
		})
	}
}

// TestRunWithFixedRequestsBuiltin 覆盖 requests>0 时按预算提前结束的路径
// （budget 命中后跳出循环，而非仅靠 duration 截止）。
func TestRunWithFixedRequestsBuiltin(t *testing.T) {
	o := &options{
		proto:    "tcp",
		mode:     "throughput",
		conns:    1,
		duration: 5 * time.Second,
		requests: 3,
		payload:  32,
		warmup:   0,
	}
	var out bytes.Buffer
	start := time.Now()
	if err := runWithOutput(o, &out, io.Discard); err != nil {
		t.Fatalf("runWithOutput: %v", err)
	}
	// 3 个请求应远早于 5s duration 完成。
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("固定请求数未提前结束: %s", elapsed)
	}
	if !strings.Contains(out.String(), "requests ok  : 3") {
		t.Fatalf("成功请求数应为 3:\n%s", out.String())
	}
}

// TestRunDelegatesToRunWithOutput 覆盖 run 的薄封装（写入真实 stdout/stderr）。
func TestRunDelegatesToRunWithOutput(t *testing.T) {
	o := &options{
		proto:    "tcp",
		mode:     "connrate",
		conns:    1,
		duration: 80 * time.Millisecond,
		requests: 0,
		payload:  32,
		warmup:   0,
	}
	if err := run(o); err != nil {
		t.Fatalf("run: %v", err)
	}
}

// TestRunBuiltinChainStartFailure 验证 startEchoTarget 失败（不支持协议）时
// runWithOutput 返回错误。proto 已在 parseFlags 校验，此处直接构造非法值以命中
// startChain 的错误传播路径。
func TestRunBuiltinChainStartFailure(t *testing.T) {
	o := &options{
		proto:    "sctp",
		mode:     "throughput",
		conns:    1,
		duration: 50 * time.Millisecond,
		payload:  32,
		warmup:   0,
	}
	if err := runWithOutput(o, io.Discard, io.Discard); err == nil {
		t.Fatal("非法协议应使内建链路启动失败并返回错误")
	}
}
