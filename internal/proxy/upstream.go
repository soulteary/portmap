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

package proxy

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	xproxy "golang.org/x/net/proxy"

	"github.com/soulteary/portmap/internal/i18n"
)

// 支持的上游代理协议。
const (
	UpstreamSchemeSOCKS5 = "socks5"
	UpstreamSchemeHTTP   = "http"
	UpstreamSchemeSSH    = "ssh"
)

// UpstreamConfig 描述一个上游代理。出站连接经此上游转发，而非直连目标。
//
// 零值（尤其是空的 Scheme）没有意义；应通过解析 -upstream URL 得到，
// 未配置上游时 Server.Upstream 应为 nil，从而保持直连行为。
type UpstreamConfig struct {
	// Scheme 是上游协议：socks5 / http / ssh。
	Scheme string

	// Addr 是上游地址，形如 "host:port"。
	Addr string

	// Username / Password 是上游认证凭据（可选）。
	// 对 socks5 为用户名/密码认证；对 http 为 Proxy-Authorization Basic；
	// 对 ssh 为登录用户名与可选密码。
	Username string
	Password string

	// IdentityFile 是 ssh 上游的私钥文件路径（可选）。
	IdentityFile string

	// IdentityPassphrase 是解密加密私钥所用的口令（可选，仅对加密私钥有意义）。
	// 为空时用 ssh.ParsePrivateKey 解析未加密私钥；非空时改用
	// ssh.ParsePrivateKeyWithPassphrase。不会进入日志或 describe()。
	IdentityPassphrase string

	// KnownHostsFile 是 ssh 上游 host key 校验使用的 known_hosts 路径；
	// 为空时默认使用 ~/.ssh/known_hosts。
	KnownHostsFile string

	// Insecure 为 true 时跳过 ssh 上游 host key 校验（不安全）。
	Insecure bool

	// KeepaliveInterval 是 ssh 上游主动保活探测的间隔（可选）。
	//   - 0：沿用默认间隔（defaultKeepaliveInterval，30s）；
	//   - 负数：禁用主动保活，退回到「拨号失败时被动重连一次」的行为；
	//   - 正数：每隔该间隔发送一次 keepalive@openssh.com 探测。
	KeepaliveInterval time.Duration

	// KeepaliveMaxFailures 是连续保活探测失败多少次后判定连接已断、
	// 触发后台重连（可选，0 表示沿用默认 defaultKeepaliveMaxFailures，3 次）。
	KeepaliveMaxFailures int
}

// describe 返回用于日志展示的上游摘要（不含凭据）。
func (c *UpstreamConfig) describe() string {
	return fmt.Sprintf("%s://%s", c.Scheme, c.Addr)
}

// ParseUpstreamURL 将形如 socks5://user:pass@host:1080 的 URL 解析为
// UpstreamConfig。SSH 专用字段（私钥、known_hosts、insecure）由调用方补齐。
func ParseUpstreamURL(raw string) (*UpstreamConfig, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamParse), err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case UpstreamSchemeSOCKS5, UpstreamSchemeHTTP, UpstreamSchemeSSH:
	default:
		return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamScheme, u.Scheme))
	}
	if u.Host == "" {
		return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamEmptyHost))
	}

	cfg := &UpstreamConfig{
		Scheme: scheme,
		Addr:   defaultUpstreamPort(scheme, u.Host),
	}
	if u.User != nil {
		cfg.Username = u.User.Username()
		if pw, ok := u.User.Password(); ok {
			cfg.Password = pw
		}
	}
	return cfg, nil
}

// defaultUpstreamPort 在 host 缺省端口时补齐各协议的默认端口。
func defaultUpstreamPort(scheme, host string) string {
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	switch scheme {
	case UpstreamSchemeSOCKS5:
		return net.JoinHostPort(host, "1080")
	case UpstreamSchemeHTTP:
		return net.JoinHostPort(host, "3128")
	case UpstreamSchemeSSH:
		return net.JoinHostPort(host, "22")
	default:
		return host
	}
}

// NewUpstreamDialer 按上游配置构造对应的 Dialer。
//
// 返回的 Dialer 可能实现 io.Closer（如 ssh 上游持有长连接），
// Server 在关闭时会检测并调用其 Close。logger 可为 nil（使用标准库默认）。
func NewUpstreamDialer(cfg *UpstreamConfig, timeout, keepAlive time.Duration, logger *log.Logger) (Dialer, error) {
	base := &net.Dialer{Timeout: timeout, KeepAlive: keepAlive}
	switch cfg.Scheme {
	case UpstreamSchemeSOCKS5:
		return newSOCKS5Dialer(cfg, base)
	case UpstreamSchemeHTTP:
		return &httpConnectDialer{cfg: cfg, base: base}, nil
	case UpstreamSchemeSSH:
		return newSSHDialer(cfg, timeout, logger)
	default:
		return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamScheme, cfg.Scheme))
	}
}

// ---- SOCKS5 上游 ----

// newSOCKS5Dialer 用 golang.org/x/net/proxy 构造 SOCKS5 上游拨号器，并包装
// 为支持 DialContext 的 Dialer。
func newSOCKS5Dialer(cfg *UpstreamConfig, base *net.Dialer) (Dialer, error) {
	var auth *xproxy.Auth
	if cfg.Username != "" || cfg.Password != "" {
		auth = &xproxy.Auth{User: cfg.Username, Password: cfg.Password}
	}
	d, err := xproxy.SOCKS5("tcp", cfg.Addr, auth, base)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSocks5), err)
	}
	return &contextDialer{d: d}, nil
}

// contextDialer 把 proxy.Dialer 适配到 Dialer 接口。x/net 的 SOCKS5 拨号器
// 通常实现 proxy.ContextDialer，优先使用其 DialContext 以支持取消。
type contextDialer struct {
	d xproxy.Dialer
}

func (c *contextDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if cd, ok := c.d.(xproxy.ContextDialer); ok {
		return cd.DialContext(ctx, network, address)
	}
	// 回退：在 goroutine 中拨号并遵守 ctx 取消。
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		conn, err := c.d.Dial(network, address)
		ch <- result{conn, err}
	}()
	select {
	case <-ctx.Done():
		go func() {
			if r := <-ch; r.conn != nil {
				_ = r.conn.Close()
			}
		}()
		return nil, ctx.Err()
	case r := <-ch:
		return r.conn, r.err
	}
}

// ---- HTTP CONNECT 上游 ----

// httpConnectDialer 对上游发起 HTTP CONNECT，隧道建立后返回底层连接。纯标准库。
type httpConnectDialer struct {
	cfg  *UpstreamConfig
	base *net.Dialer
}

func (h *httpConnectDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := h.base.DialContext(ctx, "tcp", h.cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamHTTPConnect), address, err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: make(http.Header),
	}
	if h.cfg.Username != "" || h.cfg.Password != "" {
		req.Header.Set("Proxy-Authorization", "Basic "+basicAuth(h.cfg.Username, h.cfg.Password))
	}
	if err := req.Write(conn); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamHTTPConnect), address, err)
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamHTTPConnect), address, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamHTTPStatus, address, resp.Status))
	}

	// 清除握手期间设置的绝对截止时间，交回给上层的空闲超时管理。
	_ = conn.SetDeadline(time.Time{})

	// 若 CONNECT 响应后 bufio 预读了多余字节，用包装连接把它们补回。
	if reader.Buffered() > 0 {
		buffered, _ := reader.Peek(reader.Buffered())
		return &prefixConn{Conn: conn, prefix: append([]byte(nil), buffered...)}, nil
	}
	return conn, nil
}

// prefixConn 在读取底层连接前先吐出预读缓冲的字节。
type prefixConn struct {
	net.Conn
	prefix []byte
}

func (p *prefixConn) Read(b []byte) (int, error) {
	if len(p.prefix) > 0 {
		n := copy(b, p.prefix)
		p.prefix = p.prefix[n:]
		return n, nil
	}
	return p.Conn.Read(b)
}

func basicAuth(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

// ---- SSH 上游 ----

// SSH 上游主动保活与后台重连的默认参数，与 superviseSSH 对齐
// （等价于 ssh -o ServerAliveInterval=30 -o ServerAliveCountMax=3）。
const (
	// defaultKeepaliveInterval 是保活探测的默认间隔。
	defaultKeepaliveInterval = 30 * time.Second
	// defaultKeepaliveMaxFailures 是判定连接已断前允许的连续探测失败次数。
	defaultKeepaliveMaxFailures = 3
	// defaultMinBackoff / defaultMaxBackoff 是后台重连的指数退避上下界。
	defaultMinBackoff = 1 * time.Second
	defaultMaxBackoff = 30 * time.Second
)

// sshDialer 通过一个惰性建立、断开自动重连的 *ssh.Client 打开出站连接。
//
// 首次成功建连后会启动一个后台守护 goroutine（superviseClient），周期性发送
// keepalive@openssh.com 探测；连续失败达阈值或底层连接结束时，按指数退避在后台
// 主动重连，避免 NAT/防火墙静默断连时长时间不可用。守护 goroutine 严格随 Close
// 关闭的 done 通道退出，不会泄漏。
type sshDialer struct {
	cfg     *UpstreamConfig
	timeout time.Duration
	logger  *log.Logger
	sshCfg  *ssh.ClientConfig

	// 保活/退避参数，来自配置或默认值；负的 keepaliveInterval 表示禁用主动保活。
	keepaliveInterval    time.Duration
	keepaliveMaxFailures int
	minBackoff           time.Duration
	maxBackoff           time.Duration

	mu      sync.Mutex
	client  *ssh.Client
	dialing chan struct{}
	closed  bool
	// revalidating identifies the cached client currently being probed after a
	// caller timeout when active keepalives are disabled.
	revalidating *ssh.Client

	// superviseOnce 确保只启动一个守护 goroutine。
	superviseOnce sync.Once
	// done 在 Close 时关闭，通知守护 goroutine 优雅退出。
	done chan struct{}
	// lifecycleCtx 在 Close 时取消，使进行中的 TCP/SSH 握手立即退出。
	lifecycleCtx    context.Context
	lifecycleCancel context.CancelFunc
}

// parsePrivateKey 解析 SSH 私钥：passphrase 非空时用带口令解析，否则解析未加密
// 私钥。当私钥已加密但未提供 passphrase 时，ssh.ParsePrivateKey 会返回
// *ssh.PassphraseMissingError，此处以 errors.As 识别并返回更明确的 i18n 错误，
// 提示需要提供 passphrase。passphrase 本身不会进入任何错误信息或日志。
func parsePrivateKey(key []byte, passphrase string) (ssh.Signer, error) {
	if passphrase != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
		if err != nil {
			return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHParseKey), err)
		}
		return signer, nil
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamSSHPassphraseMissing))
		}
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHParseKey), err)
	}
	return signer, nil
}

func newSSHDialer(cfg *UpstreamConfig, timeout time.Duration, logger *log.Logger) (Dialer, error) {
	var authMethods []ssh.AuthMethod

	if cfg.IdentityFile != "" {
		key, err := os.ReadFile(cfg.IdentityFile)
		if err != nil {
			return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHIdentity), cfg.IdentityFile, err)
		}
		signer, err := parsePrivateKey(key, cfg.IdentityPassphrase)
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}
	if len(authMethods) == 0 {
		return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamSSHNoAuth))
	}

	hostKeyCallback, err := sshHostKeyCallback(cfg, logger)
	if err != nil {
		return nil, err
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}

	keepaliveInterval := cfg.KeepaliveInterval
	if keepaliveInterval == 0 {
		keepaliveInterval = defaultKeepaliveInterval
	}
	keepaliveMaxFailures := cfg.KeepaliveMaxFailures
	if keepaliveMaxFailures <= 0 {
		keepaliveMaxFailures = defaultKeepaliveMaxFailures
	}

	lifecycleCtx, lifecycleCancel := context.WithCancel(context.Background())
	return &sshDialer{
		cfg:                  cfg,
		timeout:              timeout,
		logger:               logger,
		sshCfg:               sshCfg,
		keepaliveInterval:    keepaliveInterval,
		keepaliveMaxFailures: keepaliveMaxFailures,
		minBackoff:           defaultMinBackoff,
		maxBackoff:           defaultMaxBackoff,
		done:                 make(chan struct{}),
		lifecycleCtx:         lifecycleCtx,
		lifecycleCancel:      lifecycleCancel,
	}, nil
}

// sshHostKeyCallback 按配置返回 host key 校验回调：默认使用 known_hosts，
// Insecure 时跳过校验并打印醒目告警。
func sshHostKeyCallback(cfg *UpstreamConfig, logger *log.Logger) (ssh.HostKeyCallback, error) {
	if cfg.Insecure {
		logLine(logger, i18n.T(i18n.KeyLogProxyUpstreamInsecure))
		return ssh.InsecureIgnoreHostKey(), nil
	}
	path := cfg.KnownHostsFile
	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home + "/.ssh/known_hosts"
		}
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHKnownHosts), path, err)
	}
	return cb, nil
}

func (s *sshDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	client, err := s.getClient(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := s.dialThrough(ctx, client, network, address)
	if err == nil {
		return conn, nil
	}
	if ctx.Err() != nil {
		if s.keepaliveInterval < 0 {
			s.revalidateAfterTimeout(client)
		}
		return nil, ctx.Err()
	}
	// direct-tcpip 可能仅因目标拒绝连接而失败；先探测复用连接本身。连接仍
	// 健康时保留它，避免中断承载于同一 SSH 会话上的其它活跃通道。
	if sshClientHealthy(ctx, client) {
		return nil, err
	}
	if ctx.Err() != nil {
		// SendRequest cannot be canceled independently. Closing the transport
		// releases the probe goroutine before returning the caller's error.
		s.discard(client)
		return nil, ctx.Err()
	}

	// SSH 传输已断开：丢弃并重连一次后重试。
	s.discard(client)
	s.logMessage(i18n.T(i18n.KeyLogProxyUpstreamSSHReconnect), s.cfg.Addr)
	client, dialErr := s.getClient(ctx)
	if dialErr != nil {
		return nil, dialErr
	}
	return s.dialThrough(ctx, client, network, address)
}

// revalidateAfterTimeout asynchronously checks a possibly blackholed cached
// transport in passive mode. Only one probe per client may run at a time.
func (s *sshDialer) revalidateAfterTimeout(client *ssh.Client) {
	s.mu.Lock()
	if s.closed || s.client != client || s.revalidating == client {
		s.mu.Unlock()
		return
	}
	s.revalidating = client
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(s.lifecycleCtx, s.healthProbeTimeout())
		healthy := sshClientHealthy(ctx, client)
		cancel()
		if !healthy {
			s.discard(client)
		}
		s.mu.Lock()
		if s.revalidating == client {
			s.revalidating = nil
		}
		s.mu.Unlock()
	}()
}

func (s *sshDialer) healthProbeTimeout() time.Duration {
	if s.timeout > 0 {
		return s.timeout
	}
	return defaultKeepaliveInterval
}

// sshClientHealthy 通过全局请求区分“目标通道打开失败”和“SSH 传输断开”。
// 服务端即使不支持该请求也会返回 reply=false、err=nil，仍足以证明传输可用。
func sshClientHealthy(ctx context.Context, client *ssh.Client) bool {
	result := make(chan error, 1)
	go func() {
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		result <- err
	}()
	select {
	case err := <-result:
		return err == nil
	case <-ctx.Done():
		return false
	}
}

// dialThrough 在 ssh 客户端上打开到 address 的通道，并遵守 ctx 取消。
func (s *sshDialer) dialThrough(ctx context.Context, client *ssh.Client, network, address string) (net.Conn, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		conn, err := client.Dial(network, address)
		ch <- result{conn, err}
	}()
	select {
	case <-ctx.Done():
		go func() {
			if r := <-ch; r.conn != nil {
				_ = r.conn.Close()
			}
		}()
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHChannel), address, r.err)
		}
		return r.conn, nil
	}
}

// getClient 返回可用的 ssh 客户端，必要时建立连接。同一时刻只允许一次握手；
// 其它调用者等待该结果，但可由自己的 ctx 取消。
func (s *sshDialer) getClient(ctx context.Context) (*ssh.Client, error) {
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamClosed))
		}
		if s.client != nil {
			client := s.client
			s.mu.Unlock()
			return client, nil
		}
		if s.dialing != nil {
			dialing := s.dialing
			s.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-s.done:
				return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamClosed))
			case <-dialing:
				continue
			}
		}
		dialing := make(chan struct{})
		s.dialing = dialing
		s.mu.Unlock()

		client, err := s.dialClient(ctx)

		s.mu.Lock()
		if err == nil && !s.closed {
			s.client = client
		}
		closed := s.closed
		s.dialing = nil
		close(dialing)
		s.mu.Unlock()

		if closed {
			if client != nil {
				_ = client.Close()
			}
			return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamClosed))
		}
		if err != nil {
			return nil, err
		}

		s.logMessage(i18n.T(i18n.KeyLogProxyUpstreamSSHConnect), s.cfg.Addr)
		// 首次成功建连后启动后台守护：主动保活 + 断线指数退避重连。
		// keepaliveInterval < 0 表示禁用主动保活，退回被动重连行为。
		if s.keepaliveInterval > 0 {
			s.superviseOnce.Do(func() {
				go s.superviseClient(client)
			})
		}
		return client, nil
	}
}

// dialClient 分离 TCP 拨号与 SSH 握手，使两阶段都服从请求 ctx、拨号超时以及
// dialer Close。握手期间使用连接截止时间，并在返回前清除。
func (s *sshDialer) dialClient(ctx context.Context) (*ssh.Client, error) {
	dialCtx, cancel := context.WithCancel(ctx)
	stopLifecycle := context.AfterFunc(s.lifecycleCtx, cancel)
	defer func() {
		stopLifecycle()
		cancel()
	}()

	raw, err := (&net.Dialer{Timeout: s.timeout}).DialContext(dialCtx, "tcp", s.cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHDial), s.cfg.Addr, err)
	}

	if deadline, ok := dialCtx.Deadline(); ok {
		_ = raw.SetDeadline(deadline)
	} else if s.timeout > 0 {
		_ = raw.SetDeadline(time.Now().Add(s.timeout))
	}
	stopHandshake := context.AfterFunc(dialCtx, func() { _ = raw.Close() })
	conn, chans, reqs, err := ssh.NewClientConn(raw, s.cfg.Addr, s.sshCfg)
	if !stopHandshake() {
		_ = raw.Close()
	}
	if err != nil {
		_ = raw.Close()
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHDial), s.cfg.Addr, err)
	}
	_ = raw.SetDeadline(time.Time{})
	return ssh.NewClient(conn, chans, reqs), nil
}

// superviseClient 是随首个 ssh 客户端启动的后台守护 goroutine：
//   - 每隔 keepaliveInterval 发送一次 keepalive@openssh.com 探测；
//   - 连续失败达 keepaliveMaxFailures 次，或底层连接结束（client.Wait 返回），
//     判定连接已断，丢弃当前客户端并在后台按指数退避重连；
//   - 通过监听 s.done（Close 时关闭）优雅退出，避免 goroutine 泄漏。
//
// 守护始终跟随「当前」客户端：重连成功后继续对新客户端保活，无需再启动新的 goroutine。
func (s *sshDialer) superviseClient(client *ssh.Client) {
	ticker := time.NewTicker(s.keepaliveInterval)
	defer ticker.Stop()

	// 每个客户端使用独立的关闭通道，切换客户端时直接切换 select 的来源，
	// 避免旧客户端的滞留通知占满共享通道并吞掉新客户端的关闭事件。
	watch := func(c *ssh.Client) <-chan struct{} {
		closed := make(chan struct{})
		go func() {
			_ = c.Wait()
			close(closed)
		}()
		return closed
	}
	waitClosed := watch(client)

	failures := 0
	for {
		select {
		case <-s.done:
			return
		case <-waitClosed:
			// 连接已结束：立即触发重连。
			newClient, ok := s.reconnect(client)
			if !ok {
				return
			}
			client = newClient
			failures = 0
			waitClosed = watch(client)
		case <-ticker.C:
			probeCtx, cancel := context.WithTimeout(s.lifecycleCtx, s.healthProbeTimeout())
			healthy := sshClientHealthy(probeCtx, client)
			timedOut := errors.Is(probeCtx.Err(), context.DeadlineExceeded)
			cancel()
			if healthy {
				failures = 0
				continue
			}
			failures++
			if timedOut {
				// A probe that produced no reply cannot be retried safely on the
				// same transport: close it to release the blocked SendRequest.
				failures = s.keepaliveMaxFailures
			}
			s.logMessage(i18n.T(i18n.KeyLogProxyUpstreamSSHKeepaliveFail), s.cfg.Addr, failures, s.keepaliveMaxFailures)
			if failures < s.keepaliveMaxFailures {
				continue
			}
			newClient, ok := s.reconnect(client)
			if !ok {
				return
			}
			client = newClient
			failures = 0
			waitClosed = watch(client)
		}
	}
}

// reconnect 丢弃当前客户端并按指数退避（minBackoff→maxBackoff）在后台重连。
// 成功时返回新客户端与 true；若期间 Close 被调用则返回 false，调用方应退出守护。
func (s *sshDialer) reconnect(client *ssh.Client) (*ssh.Client, bool) {
	s.discard(client)
	s.logMessage(i18n.T(i18n.KeyLogProxyUpstreamSSHReconnect), s.cfg.Addr)

	backoff := s.minBackoff
	for {
		select {
		case <-s.done:
			return nil, false
		default:
		}

		newClient, err := s.getClient(context.Background())
		if err == nil {
			return newClient, true
		}

		s.logMessage(i18n.T(i18n.KeyLogProxyUpstreamSSHBackoff), backoff)
		timer := time.NewTimer(backoff)
		select {
		case <-s.done:
			timer.Stop()
			return nil, false
		case <-timer.C:
		}
		backoff *= 2
		if backoff > s.maxBackoff {
			backoff = s.maxBackoff
		}
	}
}

// discard 关闭并清除当前 ssh 客户端，使下次拨号触发重连。
func (s *sshDialer) discard(client *ssh.Client) {
	s.mu.Lock()
	if s.client == client {
		s.client = nil
	}
	s.mu.Unlock()
	_ = client.Close()
}

// Close 关闭底层 ssh 客户端，实现 io.Closer，供 Server 生命周期收尾调用。
// 同时关闭 done 通道，通知后台守护 goroutine 优雅退出，避免泄漏。
func (s *sshDialer) Close() error {
	s.mu.Lock()
	client := s.client
	s.client = nil
	alreadyClosed := s.closed
	s.closed = true
	s.mu.Unlock()
	if !alreadyClosed {
		close(s.done)
		s.lifecycleCancel()
	}
	if client != nil {
		return client.Close()
	}
	return nil
}

func (s *sshDialer) logMessage(format string, args ...any) {
	logMessage(s.logger, format, args...)
}

func logMessage(logger *log.Logger, format string, args ...any) {
	if logger != nil {
		logger.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// logLine 输出一条不含格式动词的整串消息，避免 vet 对非常量格式串误报。
func logLine(logger *log.Logger, msg string) {
	if logger != nil {
		logger.Print(msg)
		return
	}
	log.Print(msg)
}
