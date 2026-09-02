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

	// KnownHostsFile 是 ssh 上游 host key 校验使用的 known_hosts 路径；
	// 为空时默认使用 ~/.ssh/known_hosts。
	KnownHostsFile string

	// Insecure 为 true 时跳过 ssh 上游 host key 校验（不安全）。
	Insecure bool
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

// sshDialer 通过一个惰性建立、断开自动重连的 *ssh.Client 打开出站连接。
type sshDialer struct {
	cfg     *UpstreamConfig
	timeout time.Duration
	logger  *log.Logger
	sshCfg  *ssh.ClientConfig

	mu     sync.Mutex
	client *ssh.Client
	closed bool
}

func newSSHDialer(cfg *UpstreamConfig, timeout time.Duration, logger *log.Logger) (Dialer, error) {
	var authMethods []ssh.AuthMethod

	if cfg.IdentityFile != "" {
		key, err := os.ReadFile(cfg.IdentityFile)
		if err != nil {
			return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHIdentity), cfg.IdentityFile, err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHParseKey), err)
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
	return &sshDialer{
		cfg:     cfg,
		timeout: timeout,
		logger:  logger,
		sshCfg:  sshCfg,
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
	client, err := s.getClient()
	if err != nil {
		return nil, err
	}
	conn, err := s.dialThrough(ctx, client, network, address)
	if err == nil {
		return conn, nil
	}

	// 连接可能已断开：丢弃并重连一次后重试。
	s.discard(client)
	s.logMessage(i18n.T(i18n.KeyLogProxyUpstreamSSHReconnect), s.cfg.Addr)
	client, dialErr := s.getClient()
	if dialErr != nil {
		return nil, dialErr
	}
	return s.dialThrough(ctx, client, network, address)
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

// getClient 返回可用的 ssh 客户端，必要时建立连接。
func (s *sshDialer) getClient() (*ssh.Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, errors.New(i18n.T(i18n.KeyErrProxyUpstreamClosed))
	}
	if s.client != nil {
		return s.client, nil
	}
	client, err := ssh.Dial("tcp", s.cfg.Addr, s.sshCfg)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrProxyUpstreamSSHDial), s.cfg.Addr, err)
	}
	s.client = client
	s.logMessage(i18n.T(i18n.KeyLogProxyUpstreamSSHConnect), s.cfg.Addr)
	return client, nil
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
func (s *sshDialer) Close() error {
	s.mu.Lock()
	client := s.client
	s.client = nil
	s.closed = true
	s.mu.Unlock()
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
