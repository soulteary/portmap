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

// Package proxy 提供同时支持 SOCKS5 与 HTTP 的应用层代理服务。
//
// 同一个监听端口通过窥探连接首字节自动区分两种协议。默认情况下所有出站
// 连接均使用纯粹的 net.Dialer 直连目标地址；配置上游（Server.Upstream）后，
// 出站连接改为经 SOCKS5 / HTTP CONNECT / SSH 上游转发。无论是否配置上游，
// 均完全忽略 HTTP_PROXY / HTTPS_PROXY / ALL_PROXY / NO_PROXY 等环境变量。
package proxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/netutil"
	"github.com/soulteary/portmap/internal/stats"
)

// 默认参数。
const (
	defaultDialTimeout      = 30 * time.Second
	defaultKeepAlive        = 30 * time.Second
	defaultMaxConns         = 256
	defaultHandshakeTimeout = 10 * time.Second
	defaultIdleTimeout      = 5 * time.Minute
)

var errServerAlreadyServing = errors.New("proxy: server is already serving")

// Server 是同时支持 SOCKS5 与 HTTP 的代理服务。
//
// 同一个监听端口通过窥探连接首字节自动区分两种协议：
//   - 首字节为 0x05 时按 SOCKS5 处理；
//   - 否则按 HTTP/HTTPS 代理处理。
type Server struct {
	// Addr 是监听地址，形如 "127.0.0.1:1080"。
	Addr string

	// DialTimeout 是出站连接超时时间。
	DialTimeout time.Duration

	// MaxConns 是允许同时处理的最大入站连接数；0 表示不限制。
	MaxConns int

	// HandshakeTimeout 限制协议探测和 HTTP/SOCKS5 握手耗时；0 表示不限制。
	HandshakeTimeout time.Duration

	// IdleTimeout 是隧道双向空闲超时；0 表示不限制。
	IdleTimeout time.Duration

	// AllowPublic 允许监听非回环地址。代理不提供身份认证，因此必须显式开启。
	AllowPublic bool

	// Upstream 配置上游代理链。为 nil 时出站连接直连目标；非 nil 时所有出站
	// 连接经该上游（SOCKS5 / HTTP CONNECT / SSH）转发。
	Upstream *UpstreamConfig

	// Logger 用于输出日志，为 nil 时使用标准库默认 logger。
	Logger *log.Logger

	// Events 为可选的结构化连接事件缓冲区（供 Web 面板展示最近连接活动）。
	// 为 nil 时所有事件写入均为零成本的空操作（EventLog.Append 对 nil 安全）。
	Events *stats.EventLog

	dialer Dialer

	mu              sync.Mutex
	listener        net.Listener
	conns           map[net.Conn]struct{}
	handshakes      map[net.Conn]struct{}
	remotes         map[net.Conn]struct{}
	closing         bool
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
	wg              sync.WaitGroup

	stats *stats.Counters
}

// New 创建一个代理服务。所有出站连接均直连，忽略环境代理。
func New(addr string) *Server {
	return &Server{
		Addr:             addr,
		DialTimeout:      defaultDialTimeout,
		MaxConns:         defaultMaxConns,
		HandshakeTimeout: defaultHandshakeTimeout,
		IdleTimeout:      defaultIdleTimeout,
		conns:            make(map[net.Conn]struct{}),
		handshakes:       make(map[net.Conn]struct{}),
		remotes:          make(map[net.Conn]struct{}),
		stats:            stats.New(),
	}
}

// Stats 返回该代理实例的统计计数器（连接/字节/拒绝/失败/uptime）。
// 延迟初始化以兼容通过零值加字段赋值构造 Server 的用法。
func (s *Server) Stats() *stats.Counters {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stats == nil {
		s.stats = stats.New()
	}
	return s.stats
}

// Snapshot 返回该代理实例的统计快照。
func (s *Server) Snapshot() stats.Snapshot { return s.Stats().Snapshot() }

// ListenAndServe 开始监听并处理连接，阻塞直到出错或被关闭。
func (s *Server) ListenAndServe() error {
	if s.DialTimeout <= 0 {
		s.DialTimeout = defaultDialTimeout
	}
	if s.dialer == nil {
		if s.Upstream != nil {
			dialer, err := NewUpstreamDialer(s.Upstream, s.DialTimeout, defaultKeepAlive, s.Logger)
			if err != nil {
				return err
			}
			s.dialer = dialer
			s.logf(i18n.T(i18n.KeyLogProxyUpstreamEnabled), s.Upstream.Scheme, s.Upstream.describe())
		} else {
			s.dialer = NewDirectDialer(s.DialTimeout, defaultKeepAlive)
		}
	}

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	if !s.AllowPublic && !isLoopbackListener(ln.Addr()) {
		_ = ln.Close()
		return errors.New(i18n.T(i18n.KeyErrProxyPublicListen, ln.Addr()))
	}

	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		_ = ln.Close()
		return net.ErrClosed
	}
	if s.listener != nil {
		s.mu.Unlock()
		_ = ln.Close()
		return errServerAlreadyServing
	}
	s.listener = ln
	if s.conns == nil {
		s.conns = make(map[net.Conn]struct{})
	}
	if s.remotes == nil {
		s.remotes = make(map[net.Conn]struct{})
	}
	if s.handshakes == nil {
		s.handshakes = make(map[net.Conn]struct{})
	}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.listener == ln {
			s.listener = nil
		}
		s.mu.Unlock()
	}()
	s.logf(i18n.T(i18n.KeyLogProxyStarted), ln.Addr())

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.logf(i18n.T(i18n.KeyLogProxyAcceptFailed), err)
			continue
		}
		if !s.trackConn(conn) {
			s.Stats().Reject()
			s.logEvent("reject", "", conn.RemoteAddr().String(), "", 0, 0, 0, 0)
			s.logf(i18n.T(i18n.KeyLogProxyConnLimit), conn.RemoteAddr(), s.MaxConns)
			_ = conn.Close()
			continue
		}
		go func() {
			defer s.untrackConn(conn)
			s.serveConn(conn)
		}()
	}
}

// Close 立即关闭监听器和所有活动连接。
func (s *Server) Close() error {
	err := s.stopAccepting()
	s.closeActiveConns()
	s.wg.Wait()
	s.closeDialer()
	return err
}

// Shutdown 停止接受新连接，取消尚未完成的握手与拨号，并等待已建立的中继
// 自然结束。上下文到期后会强制关闭剩余连接并立即返回 ctx.Err()。
func (s *Server) Shutdown(ctx context.Context) error {
	err := s.stopAccepting()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.closeDialer()
		return err
	case <-ctx.Done():
		s.closeActiveConns()
		s.closeDialer()
		return ctx.Err()
	}
}

// closeDialer 关闭持有长连接的拨号器（如 SSH 上游）。直连与无状态上游拨号器
// 不实现 io.Closer，此处为无操作，从而保持默认路径行为不变。
func (s *Server) closeDialer() {
	s.mu.Lock()
	dialer := s.dialer
	s.mu.Unlock()
	if closer, ok := dialer.(io.Closer); ok {
		_ = closer.Close()
	}
}

// serveConn 处理单个连接：先探测协议，再分发到对应处理器。
func (s *Server) serveConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	connID := s.Stats().ConnOpened()
	defer s.Stats().ConnClosed()
	ctx := context.WithValue(s.serverContext(), connIDContextKey{}, connID)
	if s.HandshakeTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.HandshakeTimeout)
		defer cancel()
		deadline, _ := ctx.Deadline()
		if !s.setHandshakeDeadline(conn, deadline) {
			return
		}
	}

	client := &netutil.IdleConn{Conn: conn}
	reader := bufio.NewReader(client)
	// Peek 一个字节用于协议探测，不会消费它。
	prefix, err := reader.Peek(1)
	if err != nil {
		s.logf(i18n.T(i18n.KeyLogProxyDetectFailed), conn.RemoteAddr(), err)
		return
	}

	clientAddr := conn.RemoteAddr().String()
	if prefix[0] == socks5Version {
		// 探测到 SOCKS5：记录 open 事件（目标在握手后由处理器补充到 close 事件）。
		s.logEvent("open", "socks5", clientAddr, "", 0, 0, 0, connID)
		// 消费掉版本字节，交给 SOCKS5 处理器（它从 NMETHODS 继续）。
		if _, err := reader.ReadByte(); err != nil {
			return
		}
		if err := s.handleSOCKS5WithReader(ctx, client, reader); err != nil {
			s.logf(i18n.T(i18n.KeyLogProxySOCKS5Failed), conn.RemoteAddr(), err)
		}
		return
	}

	// 否则按 HTTP/HTTPS 处理：记录 open 事件。
	s.logEvent("open", "http", clientAddr, "", 0, 0, 0, connID)
	if err := s.handleHTTP(ctx, client, reader); err != nil {
		s.logf(i18n.T(i18n.KeyLogProxyHTTPFailed), conn.RemoteAddr(), err)
	}
}

type connIDContextKey struct{}

func connIDFromContext(ctx context.Context) int64 {
	connID, _ := ctx.Value(connIDContextKey{}).(int64)
	return connID
}

func (s *Server) recordDialError(ctx context.Context, proto string, conn net.Conn, target string) {
	s.Stats().DialError()
	s.logEvent("dial-error", proto, conn.RemoteAddr().String(), target, 0, 0, 0, connIDFromContext(ctx))
}

// serverContext 返回所有握手与拨号共享的服务生命周期上下文。Server 仍可
// 通过零值加字段赋值构造，因此这里采用延迟初始化。
func (s *Server) serverContext() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lifecycleCtx == nil {
		s.lifecycleCtx, s.lifecycleCancel = context.WithCancel(context.Background())
		if s.closing {
			s.lifecycleCancel()
		}
	}
	return s.lifecycleCtx
}

// setHandshakeDeadline serializes the initial deadline with shutdown. This
// prevents a just-started handler from overwriting shutdown's immediate
// deadline with a later HandshakeTimeout value.
func (s *Server) setHandshakeDeadline(conn net.Conn, deadline time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	return conn.SetDeadline(deadline) == nil
}

func (s *Server) trackConn(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing || (s.MaxConns > 0 && len(s.conns) >= s.MaxConns) {
		return false
	}
	s.conns[conn] = struct{}{}
	if s.handshakes == nil {
		s.handshakes = make(map[net.Conn]struct{})
	}
	s.handshakes[conn] = struct{}{}
	s.wg.Add(1)
	return true
}

func (s *Server) untrackConn(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	delete(s.handshakes, conn)
	s.mu.Unlock()
	s.wg.Done()
}

// trackRemote registers an outbound connection before it becomes part of a
// relay. Once shutdown has started, newly completed dials are rejected so no
// connection can escape the forced-close snapshot.
func (s *Server) trackRemote(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	if s.remotes == nil {
		s.remotes = make(map[net.Conn]struct{})
	}
	s.remotes[conn] = struct{}{}
	return true
}

func (s *Server) untrackRemote(conn net.Conn) {
	s.mu.Lock()
	delete(s.remotes, conn)
	s.mu.Unlock()
}

func (s *Server) stopAccepting() error {
	s.mu.Lock()
	s.closing = true
	ln := s.listener
	cancel := s.lifecycleCancel
	// Context cancellation interrupts DNS and dialing, while an immediate
	// socket deadline interrupts clients still blocked in protocol reads. Relay
	// connections have already been removed from handshakes and may drain.
	now := time.Now()
	for conn := range s.handshakes {
		_ = conn.SetDeadline(now)
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if ln == nil {
		return nil
	}
	err := ln.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (s *Server) closeActiveConns() {
	s.mu.Lock()
	conns := make([]net.Conn, 0, len(s.conns)+len(s.remotes))
	for conn := range s.conns {
		conns = append(conns, conn)
	}
	for conn := range s.remotes {
		conns = append(conns, conn)
	}
	s.mu.Unlock()
	for _, conn := range conns {
		_ = conn.Close()
	}
}

// beginRelay 清除握手阶段的绝对截止时间，并把两个端点关联为共享的滚动
// 空闲超时：任一方向有流量都视为隧道仍然活跃。
func (s *Server) beginRelay(client, remote net.Conn) bool {
	rawClient := client
	if idle, ok := client.(*netutil.IdleConn); ok {
		rawClient = idle.Conn
	}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return false
	}
	delete(s.handshakes, rawClient)
	s.mu.Unlock()

	_ = client.SetDeadline(time.Time{})
	_ = remote.SetDeadline(time.Time{})
	clientIdle, clientOK := client.(*netutil.IdleConn)
	remoteIdle, remoteOK := remote.(*netutil.IdleConn)
	if clientOK && remoteOK {
		netutil.ShareIdleTimeout(clientIdle, remoteIdle, s.IdleTimeout)
	}
	return true
}

func (s *Server) wrapRemote(conn net.Conn) net.Conn {
	return &netutil.IdleConn{Conn: conn, Timeout: s.IdleTimeout}
}

func isLoopbackListener(addr net.Addr) bool {
	tcpAddr, ok := addr.(*net.TCPAddr)
	return ok && tcpAddr.IP.IsLoopback()
}

// isSelfTarget 判断目标是否会回连当前监听器，避免 HTTP/SOCKS 请求触发递归连接风暴。
func (s *Server) isSelfTarget(ctx context.Context, target string) bool {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return false
	}

	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()
	if ln == nil {
		return false
	}
	listenAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || port != fmt.Sprint(listenAddr.Port) {
		return false
	}

	host = strings.Trim(host, "[]")
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		resolved, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil {
			return false
		}
		for _, ip := range resolved {
			ips = append(ips, net.IP(ip.AsSlice()))
		}
	}

	for _, ip := range ips {
		if listenAddr.IP.IsUnspecified() {
			if isLocalIP(ip) {
				return true
			}
			continue
		}
		if listenAddr.IP.Equal(ip) {
			return true
		}
	}
	return false
}

// isSelfConn 在拨号完成后再次检查实际对端，防止 DNS 在检查与拨号之间变化。
func (s *Server) isSelfConn(conn net.Conn) bool {
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return false
	}
	s.mu.Lock()
	ln := s.listener
	s.mu.Unlock()
	if ln == nil {
		return false
	}
	listenAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || listenAddr.Port != remoteAddr.Port {
		return false
	}
	if listenAddr.IP.IsUnspecified() {
		return isLocalIP(remoteAddr.IP)
	}
	return listenAddr.IP.Equal(remoteAddr.IP)
}

func isLocalIP(target net.IP) bool {
	if target.IsLoopback() {
		return true
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.Equal(target) {
			return true
		}
	}
	return false
}

func (s *Server) logf(format string, args ...any) {
	if s.Logger != nil {
		s.Logger.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// eventTime 返回当前时间，格式化为 stats.Event 约定的 "2006-01-02 15:04:05"。
func eventTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// logEvent 向可选的事件缓冲区追加一条结构化连接事件；对 nil s.Events 安全，
// 因此调用方无需在插桩处判空即可直接调用。
func (s *Server) logEvent(kind, proto, client, target string, up, down, durMs, connID int64) {
	s.Events.Append(stats.Event{
		Time:       eventTime(),
		Kind:       kind,
		Proto:      proto,
		Client:     client,
		Target:     target,
		UpBytes:    up,
		DownBytes:  down,
		DurationMs: durMs,
		ConnID:     connID,
	})
}
