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

// Package forward 实现等价于 socat TCP-LISTEN:<port>,fork,reuseaddr TCP:<host:port>
// 的纯 Go 端口转发逻辑：监听本地端口，对每个新连接并发处理（fork），
// 并将字节流双向转发到目标地址。支持 TCP 与 UDP。
package forward

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/netutil"
	"github.com/soulteary/portmap/internal/stats"
)

// Config 描述一次转发服务的配置。
type Config struct {
	// Listen 是本地监听地址，例如 ":22" 或 "0.0.0.0:22"。
	Listen string
	// Target 是转发目标地址，例如 "127.0.0.1:2222"。
	Target string
	// Network 是使用的网络类型，"tcp"（默认）或 "udp"。
	Network string
	// ReuseAddr 对应 socat 的 reuseaddr 选项，启用 SO_REUSEADDR。
	ReuseAddr bool
	// DialTimeout 拨号到目标地址的超时时间，0 表示使用默认值。
	DialTimeout time.Duration
	// MaxConns 限制同时处理的连接数，0 表示不限制。TCP 下超限时新连接会排队等待；
	// UDP 下超限时直接拒绝新会话（无排队语义）。
	MaxConns int
	// IdleTimeout 空闲超时：在此时间内双向均无数据则断开连接，0 表示不启用。
	// UDP 下 0 表示回退为默认 60s 回收空闲会话。
	IdleTimeout time.Duration
	// Logger 用于输出日志，nil 时使用标准库默认 logger。
	Logger *log.Logger
	// Debug 为 true 时输出更详细的调试日志（含 pipe 错误、每连接细节）。
	Debug bool
	// Quiet 为 true 时抑制每连接的常规日志，只保留告警/错误。
	Quiet bool
	// Events 为可选的结构化连接事件缓冲区（供 Web 面板展示最近连接活动）。
	// 为 nil 时所有事件写入均为零成本的空操作（EventLog.Append 对 nil 安全）。
	Events *stats.EventLog
}

// Server 是一个端口转发服务。
type Server struct {
	cfg    Config
	logger *log.Logger

	// sem 用于并发限流，nil 表示不限制。
	sem chan struct{}

	wg    sync.WaitGroup
	stats *stats.Counters // 运行时统计（连接/字节/拒绝/失败/uptime）
}

// New 根据配置创建一个转发服务。
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 10 * time.Second
	}
	if strings.TrimSpace(cfg.Network) == "" {
		cfg.Network = "tcp"
	}
	s := &Server{cfg: cfg, logger: logger, stats: stats.New()}
	if cfg.MaxConns > 0 {
		s.sem = make(chan struct{}, cfg.MaxConns)
	}
	return s
}

// ActiveConns 返回当前活跃连接数。
func (s *Server) ActiveConns() int64 { return s.stats.ActiveConns() }

// TotalConns 返回累计处理的连接数。
func (s *Server) TotalConns() int64 { return s.stats.TotalConns() }

// Stats 返回该实例的统计计数器，供快照/日志/HTTP 端点读取。
func (s *Server) Stats() *stats.Counters { return s.stats }

// Snapshot 返回该实例的统计快照。
func (s *Server) Snapshot() stats.Snapshot { return s.stats.Snapshot() }

// infof 输出常规信息日志（受 Quiet 抑制）。
func (s *Server) infof(format string, args ...any) {
	if s.cfg.Quiet {
		return
	}
	s.logger.Printf(format, args...)
}

// debugf 输出调试日志（仅 Debug 时）。
func (s *Server) debugf(format string, args ...any) {
	if !s.cfg.Debug {
		return
	}
	s.logger.Printf("[debug] "+format, args...)
}

// warnf 输出告警/错误日志（不受 Quiet 抑制）。
func (s *Server) warnf(format string, args ...any) {
	s.logger.Printf(format, args...)
}

// listenConfig 构造带 SO_REUSEADDR 控制的 net.ListenConfig。
func (s *Server) listenConfig() net.ListenConfig {
	return net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			if !s.cfg.ReuseAddr {
				return nil
			}
			var ctrlErr error
			err := c.Control(func(fd uintptr) {
				ctrlErr = setReuseAddr(fd)
			})
			if err != nil {
				return err
			}
			return ctrlErr
		},
	}
}

// ListenAndServe 开始监听并接受连接，直到 ctx 被取消或发生致命错误。
func (s *Server) ListenAndServe(ctx context.Context) error {
	switch s.cfg.Network {
	case "tcp":
		return s.serveTCP(ctx)
	case "udp":
		return s.serveUDP(ctx)
	default:
		return fmt.Errorf(i18n.T(i18n.KeyErrUnsupportedNet), s.cfg.Network)
	}
}

// serveTCP 处理 TCP 监听与转发。
func (s *Server) serveTCP(ctx context.Context) error {
	lc := s.listenConfig()
	ln, err := lc.Listen(ctx, "tcp", s.cfg.Listen)
	if err != nil {
		return err
	}
	defer func() { _ = ln.Close() }()
	if targetMatchesListener(ctx, s.cfg.Target, ln.Addr()) {
		return fmt.Errorf(i18n.T(i18n.KeyErrFwdSelfTarget), s.cfg.Target)
	}
	s.infof(i18n.T(i18n.KeyLogTCPListening),
		ln.Addr(), s.cfg.Target, s.cfg.ReuseAddr, s.cfg.MaxConns, s.cfg.IdleTimeout)

	// ctx 取消时关闭 listener，使 Accept 立即返回。
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				// 正常退出：等待所有在途连接处理完毕。
				s.wg.Wait()
				return nil
			}
			// 临时性错误退避后重试。
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				time.Sleep(50 * time.Millisecond)
				continue
			}
			return err
		}

		// 并发限流：超限时排队等待（或 ctx 取消时放弃）。
		if s.sem != nil {
			select {
			case s.sem <- struct{}{}:
			case <-ctx.Done():
				_ = conn.Close()
				s.wg.Wait()
				return nil
			}
		}

		// fork：每个连接由独立的 goroutine 处理。
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if s.sem != nil {
				defer func() { <-s.sem }()
			}
			s.handle(ctx, conn, ln.Addr())
		}()
	}
}

// handle 处理单个入站 TCP 连接：拨号到目标并双向转发。
func (s *Server) handle(ctx context.Context, src net.Conn, listenAddr net.Addr) {
	defer func() { _ = src.Close() }()

	dialer := net.Dialer{Timeout: s.cfg.DialTimeout}
	dst, err := dialer.DialContext(ctx, "tcp", s.cfg.Target)
	if err != nil {
		s.stats.DialError()
		s.cfg.Events.Append(stats.Event{
			Time:   eventTime(),
			Kind:   "dial-error",
			Proto:  s.cfg.Network,
			Client: src.RemoteAddr().String(),
			Target: s.cfg.Target,
		})
		s.warnf(i18n.T(i18n.KeyLogDialFailed), s.cfg.Target, err)
		return
	}
	defer func() { _ = dst.Close() }()
	// DNS may change between the preflight check and the actual dial. Verify the
	// connected peer as well so a rebinding cannot turn a forwarder into itself.
	if connMatchesListener(dst, listenAddr) {
		s.stats.DialError()
		s.warnf(i18n.T(i18n.KeyErrFwdSelfTarget), s.cfg.Target)
		return
	}

	s.stats.ConnOpened()
	defer s.stats.ConnClosed()
	connID := s.stats.TotalConns()
	active := s.stats.ActiveConns()

	start := time.Now()
	s.infof(i18n.T(i18n.KeyLogConnOpen), connID, src.RemoteAddr(), dst.RemoteAddr(), active)
	s.cfg.Events.Append(stats.Event{
		Time:   eventTime(),
		Kind:   "open",
		Proto:  s.cfg.Network,
		Client: src.RemoteAddr().String(),
		Target: dst.RemoteAddr().String(),
		ConnID: connID,
	})

	// 可取消的子 ctx：ctx.Done 或转发结束时主动关闭两端。
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-cctx.Done()
		_ = src.Close()
		_ = dst.Close()
	}()

	var upBytes, downBytes int64
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n := s.pipe(connID, "dst<-src", dst, src)
		atomic.AddInt64(&upBytes, n)
	}()
	go func() {
		defer wg.Done()
		n := s.pipe(connID, "src<-dst", src, dst)
		atomic.AddInt64(&downBytes, n)
	}()
	wg.Wait()

	s.stats.AddUp(atomic.LoadInt64(&upBytes))
	s.stats.AddDown(atomic.LoadInt64(&downBytes))

	s.infof(i18n.T(i18n.KeyLogConnClose),
		connID, src.RemoteAddr(), dst.RemoteAddr(),
		atomic.LoadInt64(&upBytes), atomic.LoadInt64(&downBytes), time.Since(start).Round(time.Millisecond))
	s.cfg.Events.Append(stats.Event{
		Time:       eventTime(),
		Kind:       "close",
		Proto:      s.cfg.Network,
		Client:     src.RemoteAddr().String(),
		Target:     dst.RemoteAddr().String(),
		UpBytes:    atomic.LoadInt64(&upBytes),
		DownBytes:  atomic.LoadInt64(&downBytes),
		DurationMs: time.Since(start).Milliseconds(),
		ConnID:     connID,
	})
}

// pipe 将 src 的数据拷贝到 dst，返回拷贝的字节数；
// 结束后尝试半关闭 dst 的写方向，以便对端感知 EOF。
// 若配置了 IdleTimeout，则用 netutil.IdleConn 包装 src 实现空闲断开。
func (s *Server) pipe(connID int64, dir string, dst, src net.Conn) int64 {
	var reader io.Reader = src
	if s.cfg.IdleTimeout > 0 {
		reader = &netutil.IdleConn{Conn: src, Timeout: s.cfg.IdleTimeout}
	}
	n, err := netutil.CopyAndCloseWrite(dst, reader)
	if err != nil && !isNormalClose(err) {
		s.debugf(i18n.T(i18n.KeyLogPipeError), connID, dir, err)
	}
	return n
}

// eventTime 返回当前时间，格式化为 stats.Event 约定的 "2006-01-02 15:04:05"。
func eventTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// isNormalClose 判断错误是否属于连接正常关闭/取消一类，不应作为异常记录。
func isNormalClose(err error) bool {
	if err == nil {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		// 空闲超时属于预期断开。
		return true
	}
	return false
}
