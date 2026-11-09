package upstream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUpstreamDescribe(t *testing.T) {
	cfg := &UpstreamConfig{Scheme: "socks5", Addr: "127.0.0.1:1080", Username: "u", Password: "p"}
	if got := cfg.Describe(); got != "socks5://127.0.0.1:1080" {
		t.Fatalf("describe()=%q", got)
	}
}

// TestDefaultUpstreamPort 覆盖各协议默认端口及已带端口时原样返回。
func TestDefaultUpstreamPort(t *testing.T) {
	cases := []struct {
		scheme, host, want string
	}{
		{UpstreamSchemeSOCKS5, "host", "host:1080"},
		{UpstreamSchemeHTTP, "host", "host:3128"},
		{UpstreamSchemeSSH, "host", "host:22"},
		{UpstreamSchemeSOCKS5, "host:9050", "host:9050"},
		{"other", "host", "host"},
	}
	for _, c := range cases {
		if got := defaultUpstreamPort(c.scheme, c.host); got != c.want {
			t.Errorf("defaultUpstreamPort(%q,%q)=%q，期望 %q", c.scheme, c.host, got, c.want)
		}
	}
}

// TestPrefixConn 验证 prefixConn 先吐出预读前缀再读取底层连接。
func TestPrefixConn(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	pc := &prefixConn{Conn: client, prefix: []byte("HELLO")}

	// 先从前缀读取。
	buf := make([]byte, 3)
	n, err := pc.Read(buf)
	if err != nil || n != 3 || string(buf[:n]) != "HEL" {
		t.Fatalf("前缀首次读取=%q,%d,%v", buf[:n], n, err)
	}
	n, err = pc.Read(buf)
	if err != nil || string(buf[:n]) != "LO" {
		t.Fatalf("前缀剩余读取=%q,%v", buf[:n], err)
	}

	// 前缀耗尽后应读到底层连接的数据。
	go func() { _, _ = server.Write([]byte("WORLD")) }()
	got := make([]byte, 5)
	if _, err := readFull(pc, got); err != nil {
		t.Fatalf("底层读取失败: %v", err)
	}
	if string(got) != "WORLD" {
		t.Fatalf("底层读取=%q，期望 WORLD", got)
	}
}

func readFull(c net.Conn, buf []byte) (int, error) {
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	total := 0
	for total < len(buf) {
		n, err := c.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// TestBasicAuth 验证 Basic 认证编码正确。
func TestBasicAuth(t *testing.T) {
	// "alice:secret" 的 base64 是 YWxpY2U6c2VjcmV0。
	if got := basicAuth("alice", "secret"); got != "YWxpY2U6c2VjcmV0" {
		t.Fatalf("basicAuth=%q", got)
	}
}

// TestContextDialerFallbackDial 验证当底层 proxy.Dialer 仅实现 Dial（无
// ContextDialer）时，contextDialer 走回退分支并能正常返回连接。
func TestContextDialerFallbackDial(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	cd := &contextDialer{d: plainDialer{}}
	conn, err := cd.DialContext(context.Background(), "tcp", backendAddr)
	if err != nil {
		t.Fatalf("回退拨号失败: %v", err)
	}
	_ = conn.Close()
}

// TestContextDialerFallbackCancel 验证回退分支遵守 ctx 取消。
func TestContextDialerFallbackCancel(t *testing.T) {
	cd := &contextDialer{d: blockingPlainDialer{release: make(chan struct{})}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := cd.DialContext(ctx, "tcp", "127.0.0.1:1"); err == nil {
		t.Fatal("已取消的 ctx 应返回错误")
	}
}

// plainDialer 仅实现 proxy.Dialer.Dial（不实现 ContextDialer）。
type plainDialer struct{}

func (plainDialer) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}

// blockingPlainDialer 的 Dial 阻塞直到 release 关闭，用于验证 ctx 取消。
type blockingPlainDialer struct {
	release chan struct{}
}

func (b blockingPlainDialer) Dial(network, addr string) (net.Conn, error) {
	<-b.release
	return nil, errors.New("released")
}

func TestUpstreamClosedStateAndDeadlineAdapters(t *testing.T) {
	raw := &memoryConn{}
	ch := &sshChanConn{Conn: raw}
	if err := ch.SetWriteDeadline(time.Time{}); err != nil {
		t.Fatal(err)
	}
	if err := ch.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	if !raw.closed {
		t.Fatal("fallback close lost")
	}
	k := &agentKeyring{closed: true}
	if _, err := k.signers(); err == nil {
		t.Fatal("closed keyring accepted")
	}
	raw = &memoryConn{}
	if !k.swap(raw) || !raw.closed {
		t.Fatal("closed keyring leaked replacement")
	}
	s := &sshDialer{closed: true}
	if _, err := s.getClient(context.Background()); err == nil {
		t.Fatal("closed SSH dialer accepted")
	}
	if s.healthProbeTimeout() != defaultKeepaliveInterval {
		t.Fatal("invalid fallback probe timeout")
	}
	s.revalidateAfterTimeout(nil)
	if _, err := parsePrivateKey([]byte("invalid"), ""); err == nil {
		t.Fatal("invalid private key accepted")
	}
	logMessage(nil, "%s", "fallback logger")
	logLine(nil, "fallback line logger")
}

func oneShotUpstream(t *testing.T, reply string) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_ = conn.SetDeadline(time.Now().Add(time.Second))
		_, _ = io.WriteString(conn, reply)
		_, _ = io.Copy(io.Discard, conn)
	}()
	t.Cleanup(func() { _ = ln.Close(); <-done })
	return ln.Addr().String()
}
func TestHTTPUpstreamDeadlinePrefixAndMalformedResponse(t *testing.T) {
	for _, reply := range []string{"HTTP/1.1 200 OK\r\n\r\nprefix", "malformed\r\n\r\n"} {
		t.Run(reply, func(t *testing.T) {
			d := &httpConnectDialer{cfg: &UpstreamConfig{Addr: oneShotUpstream(t, reply)}, base: &net.Dialer{}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			conn, err := d.DialContext(ctx, "tcp", "example.test:80")
			if strings.HasPrefix(reply, "malformed") {
				if err == nil {
					t.Fatal("invalid upstream response accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.Close() }()
			buf := make([]byte, 6)
			if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "prefix" {
				t.Fatalf("buffered prefix=%q %v", buf, err)
			}
		})
	}
}
func TestHTTPUpstreamDialAndRequestErrors(t *testing.T) {
	d := &httpConnectDialer{cfg: &UpstreamConfig{Addr: "invalid"}, base: &net.Dialer{}}
	if _, err := d.DialContext(context.Background(), "tcp", "example.test:80"); err == nil {
		t.Fatal("invalid upstream accepted")
	}
	d.cfg.Addr = oneShotUpstream(t, "")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := d.DialContext(ctx, "tcp", "bad\nhost:80"); err == nil {
		t.Fatal("invalid CONNECT host accepted")
	}
	if _, err := newSOCKS5Dialer(&UpstreamConfig{Addr: "127.0.0.1:1080", Username: "user", Password: "secret"}, &net.Dialer{}); err != nil {
		t.Fatal(err)
	}
}
func TestSSHClientWaitersAreCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := &sshDialer{dialing: make(chan struct{}), done: make(chan struct{})}
	if _, err := s.getClient(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(s.done)
	if _, err := s.getClient(context.Background()); err == nil {
		t.Fatal("closed dialer waiter not released")
	}
}

func TestSSHReconnectBackoffCanBeStopped(t *testing.T) {
	key, pub := generateClientKey(t)
	addr, _, stop := startSSHServer(t, pub)
	defer stop()
	logger := &retryLog{messages: make(chan struct{}, 32)}
	raw, err := NewUpstreamDialer(&UpstreamConfig{Scheme: UpstreamSchemeSSH, Addr: addr, Username: "tester", IdentityFile: writeIdentity(t, key), Insecure: true, KeepaliveInterval: -1}, time.Second, time.Second, log.New(logger, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	s := raw.(*sshDialer)
	defer func() { _ = s.Close() }()
	s.minBackoff = time.Millisecond
	s.maxBackoff = 2 * time.Millisecond
	client, err := s.getClient(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	stop()
	for len(logger.messages) > 0 {
		<-logger.messages
	}
	done := make(chan bool, 1)
	go func() { _, ok := s.reconnect(client); done <- ok }()
	for i := 0; i < 4; i++ {
		select {
		case <-logger.messages:
		case <-time.After(3 * time.Second):
			t.Fatal("reconnect did not retry")
		}
	}
	_ = s.Close()
	select {
	case ok := <-done:
		if ok {
			t.Fatal("closed reconnect succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("reconnect ignored close")
	}
	if _, ok := s.reconnect(client); ok {
		t.Fatal("already closed reconnect succeeded")
	}
}

type retryLog struct{ messages chan struct{} }

func (w *retryLog) Write(p []byte) (int, error) {
	select {
	case w.messages <- struct{}{}:
	default:
	}
	return len(p), nil
}

type memoryConn struct {
	input       io.Reader
	output      bytes.Buffer
	remote      net.Addr
	writeErr    error
	deadlineErr error
	closed      bool
}

func (c *memoryConn) Read(p []byte) (int, error) {
	if c.input == nil {
		return 0, io.EOF
	}
	return c.input.Read(p)
}
func (c *memoryConn) Write(p []byte) (int, error) {
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	return c.output.Write(p)
}
func (c *memoryConn) Close() error { c.closed = true; return nil }
func (c *memoryConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}
}
func (c *memoryConn) RemoteAddr() net.Addr {
	if c.remote != nil {
		return c.remote
	}
	return &net.TCPAddr{IP: net.IPv4(192, 0, 2, 1), Port: 80}
}
func (c *memoryConn) SetDeadline(time.Time) error      { return c.deadlineErr }
func (c *memoryConn) SetReadDeadline(time.Time) error  { return c.deadlineErr }
func (c *memoryConn) SetWriteDeadline(time.Time) error { return c.deadlineErr }

func startBackend(t *testing.T) (string, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello from backend")
	})
	srv := &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("后端监听失败: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	return ln.Addr().String(), func() { _ = srv.Close() }
}

func TestUpstreamParseErrorRedactsCredentials(t *testing.T) {
	for _, raw := range []string{"ssh://user:private-value@host:bad", "ssh://user:%private-value@host"} {
		_, err := ParseUpstreamURL(raw)
		if err == nil || strings.Contains(err.Error(), "private-value") {
			t.Fatalf("unsafe parse error: %v", err)
		}
	}
	cfg := &UpstreamConfig{Scheme: "ssh", Addr: "server.example:22", Username: "private-user", Password: "private-password", IdentityPassphrase: "private-passphrase"}
	if got := cfg.Describe(); got != "ssh://server.example:22" {
		t.Fatalf("unsafe upstream description: %q", got)
	}
}
