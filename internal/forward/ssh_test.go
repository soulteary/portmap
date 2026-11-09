package forward

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/stats"
	"github.com/soulteary/portmap/internal/upstream"
)

// startSSHForward reserves the listener before startup, avoiding readiness probes
// that would themselves create SSH channels and affect connection assertions.
func startSSHForward(t *testing.T, cfg Config) (string, *Server, context.CancelFunc, <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Listen = ln.Addr().String()
	if cfg.Target == "" {
		cfg.Target = cfg.Listen
	}
	cfg.Logger = log.New(io.Discard, "", 0)
	srv := New(cfg)
	if err := srv.initDialer(); err != nil {
		_ = ln.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		child, stop := context.WithCancel(ctx)
		defer stop()
		done <- srv.serveTCPListener(ctx, child, stop, ln)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("forwarder failed to stop")
		}
	})
	return cfg.Listen, srv, cancel, done
}

func TestSSHForwardHTTPAndConcurrentChannels(t *testing.T) {
	backend := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "remote application") })}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = backend.Serve(ln) }()
	t.Cleanup(func() { _ = backend.Close() })
	targets := make(chan string, 32)
	ssh := StartSSH(t, func(_ context.Context, target string) (string, error) {
		targets <- target
		return ln.Addr().String(), nil
	})
	events := stats.NewEventLog(100)
	addr, srv, _, _ := startSSHForward(t, Config{Upstream: ssh.Config(), IdleTimeout: time.Second, Events: events})
	client := &http.Client{Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, Timeout: 3 * time.Second}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assertForwardHTTPResponse(t, client, addr)
		}()
	}
	wg.Wait()
	if ssh.Connections.Load() != 1 {
		t.Fatalf("SSH handshakes=%d", ssh.Connections.Load())
	}
	for i := 0; i < 8; i++ {
		if target := <-targets; target != addr {
			t.Fatalf("remote target=%q want=%q", target, addr)
		}
	}
	deadline := time.Now().Add(time.Second)
	for srv.ActiveConns() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	snap := srv.Snapshot()
	if snap.TotalConns != 8 || snap.UpBytes == 0 || snap.DownBytes == 0 {
		t.Fatalf("counters=%+v", snap)
	}
	for _, event := range events.Snapshot() {
		if event.Target != addr {
			t.Fatalf("event target=%q want=%q", event.Target, addr)
		}
	}
}

func TestSSHForwardHalfClose(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		body, _ := io.ReadAll(c)
		_, _ = c.Write(append([]byte("response:"), body...))
	}()
	ssh := StartSSH(t, nil)
	addr, _, _, _ := startSSHForward(t, Config{Target: ln.Addr().String(), Upstream: ssh.Config(), IdleTimeout: time.Second})
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = io.WriteString(c, "request")
	_ = c.(*net.TCPConn).CloseWrite()
	body, err := io.ReadAll(c)
	if err != nil || string(body) != "response:request" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}

func TestSSHForwardUpgradeBytes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		r := bufio.NewReader(c)
		_, err = http.ReadRequest(r)
		if err != nil {
			return
		}
		_, _ = io.WriteString(c, "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: echo\r\n\r\n")
		_, _ = io.Copy(c, r)
	}()
	ssh := StartSSH(t, nil)
	addr, _, _, _ := startSSHForward(t, Config{Target: ln.Addr().String(), Upstream: ssh.Config(), IdleTimeout: time.Second})
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	_ = c.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = io.WriteString(c, "GET / HTTP/1.1\r\nHost: example.test\r\nConnection: Upgrade\r\nUpgrade: echo\r\n\r\n")
	reader := bufio.NewReader(c)
	resp, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 101 {
		t.Fatal(resp.Status)
	}
	payload := []byte{0, 1, 2, 255, 13, 10}
	_, _ = c.Write(payload)
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(reader, got); err != nil || string(got) != string(payload) {
		t.Fatalf("echo=%v err=%v", got, err)
	}
}

func TestSSHForwardRefusalAndReconnect(t *testing.T) {
	echo, stop := startEchoServer(t)
	defer stop()
	var mu sync.Mutex
	reject := false
	ssh := StartSSH(t, func(_ context.Context, target string) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		if reject {
			return "", errors.New("refused")
		}
		return echo, nil
	})
	addr, _, _, _ := startSSHForward(t, Config{Target: "application.test:10024", Upstream: ssh.Config(), DialTimeout: time.Second})
	dial := func() net.Conn {
		t.Helper()
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		_ = c.SetDeadline(time.Now().Add(3 * time.Second))
		return c
	}
	first := dial()
	defer func() { _ = first.Close() }()
	echoOnce := func(c net.Conn) {
		t.Helper()
		_, _ = c.Write([]byte("x"))
		buf := make([]byte, 1)
		if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "x" {
			t.Fatalf("echo=%q err=%v", buf, err)
		}
	}
	echoOnce(first)
	mu.Lock()
	reject = true
	mu.Unlock()
	second := dial()
	_, err := second.Read(make([]byte, 1))
	_ = second.Close()
	if err == nil {
		t.Fatal("refused target succeeded")
	}
	echoOnce(first)
	if ssh.Connections.Load() != 1 {
		t.Fatal("target refusal discarded healthy transport")
	}
	mu.Lock()
	reject = false
	mu.Unlock()
	ssh.DropConnections()
	third := dial()
	defer func() { _ = third.Close() }()
	echoOnce(third)
	if ssh.Connections.Load() != 2 {
		t.Fatalf("handshakes after reconnect=%d", ssh.Connections.Load())
	}
}

func TestSSHForwardIdleTimeoutIsolatesChannels(t *testing.T) {
	echo, stop := startEchoServer(t)
	defer stop()
	ssh := StartSSH(t, nil)
	addr, _, _, _ := startSSHForward(t, Config{Target: echo, Upstream: ssh.Config(), IdleTimeout: 180 * time.Millisecond})
	idle, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = idle.Close() }()
	busy, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = busy.Close() }()
	_ = busy.SetDeadline(time.Now().Add(3 * time.Second))
	// Activity in one direction keeps the whole active tunnel alive.
	for i := 0; i < 12; i++ {
		_, _ = busy.Write([]byte("a"))
		var b [1]byte
		if _, err := io.ReadFull(busy, b[:]); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Millisecond)
	}
	_ = idle.SetReadDeadline(time.Now().Add(time.Second))
	_, err = idle.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("idle channel remained open")
	}
	if e, ok := err.(net.Error); ok && e.Timeout() {
		t.Fatal("client timed out instead of channel closing")
	}
	if ssh.Connections.Load() != 1 {
		t.Fatal("idle channel closed the SSH transport")
	}
}

func TestSSHForwardAuthenticationFailsClosed(t *testing.T) {
	echo, stop := startEchoServer(t)
	defer stop()
	ssh := StartSSH(t, nil)
	other := StartSSH(t, nil)
	for _, kind := range []string{"host-key", "authentication"} {
		t.Run(kind, func(t *testing.T) {
			cfg := ssh.Config()
			if kind == "host-key" {
				data, err := os.ReadFile(other.KnownHosts)
				if err != nil {
					t.Fatal(err)
				}
				data = []byte(strings.ReplaceAll(string(data), other.Addr, ssh.Addr))
				cfg.KnownHostsFile = t.TempDir() + "/known_hosts"
				if err := os.WriteFile(cfg.KnownHostsFile, data, 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				cfg.IdentityFile = other.Identity
			}
			addr, _, _, _ := startSSHForward(t, Config{Target: echo, Upstream: cfg, DialTimeout: time.Second})
			c, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = c.Close() }()
			_ = c.SetDeadline(time.Now().Add(2 * time.Second))
			_, _ = c.Write([]byte("must not echo"))
			if n, err := c.Read(make([]byte, 32)); n != 0 || err == nil {
				t.Fatalf("SSH failure fell back to direct: n=%d err=%v", n, err)
			}
		})
	}
}

func TestSSHForwardPendingChannelStopsOnCancellation(t *testing.T) {
	entered := make(chan struct{}, 1)
	ssh := StartSSH(t, func(ctx context.Context, _ string) (string, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return "", ctx.Err()
	})
	addr, _, cancel, _ := startSSHForward(t, Config{Target: "application.test:10024", Upstream: ssh.Config(), DialTimeout: time.Hour})
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("channel open did not start")
	}
	cancel()
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Fatal("pending connection survived cancellation")
	}
}

func TestSSHForwardInvalidConfig(t *testing.T) {
	for _, cfg := range []Config{{Network: "udp", Upstream: &upstream.UpstreamConfig{Scheme: "ssh"}}, {Upstream: &upstream.UpstreamConfig{Scheme: "http"}}, {Upstream: &upstream.UpstreamConfig{Scheme: "ssh", DisableAgent: true}}} {
		if err := New(cfg).ListenAndServe(context.Background()); err == nil {
			t.Fatalf("invalid config accepted: %s", fmt.Sprint(cfg.Network))
		}
	}
}

func TestSSHForwardDialTimeoutAndLateChannelCleanup(t *testing.T) {
	echo, stop := startEchoServer(t)
	defer stop()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	ssh := StartSSH(t, func(ctx context.Context, _ string) (string, error) {
		first := false
		once.Do(func() { first = true; close(entered) })
		if first {
			select {
			case <-release:
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		return echo, nil
	})
	addr, _, _, _ := startSSHForward(t, Config{Target: "application.test:10024", Upstream: ssh.Config(), DialTimeout: 120 * time.Millisecond})
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("channel did not open")
	}
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := c.Read(make([]byte, 1)); err == nil {
		t.Fatal("dial deadline did not close local connection")
	} else if e, ok := err.(net.Error); ok && e.Timeout() {
		t.Fatal("client deadline expired before forward dial deadline")
	}
	close(release)
	next, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = next.Close() }()
	_ = next.SetDeadline(time.Now().Add(2 * time.Second))
	_, _ = next.Write([]byte("ok"))
	buf := make([]byte, 2)
	if _, err := io.ReadFull(next, buf); err != nil || string(buf) != "ok" {
		t.Fatalf("subsequent channel=%q err=%v", buf, err)
	}
	if ssh.Connections.Load() != 1 {
		t.Fatal("channel timeout discarded healthy transport")
	}
}

func TestSSHForwardInstancesOwnTheirTransports(t *testing.T) {
	echo, stop := startEchoServer(t)
	defer stop()
	ssh := StartSSH(t, nil)
	cfg := Config{Target: echo, Upstream: ssh.Config()}
	first, _, cancel, _ := startSSHForward(t, cfg)
	second, _, _, _ := startSSHForward(t, cfg)
	dial := func(addr string) net.Conn {
		t.Helper()
		c, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		_ = c.SetDeadline(time.Now().Add(2 * time.Second))
		return c
	}
	a := dial(first)
	defer func() { _ = a.Close() }()
	b := dial(second)
	defer func() { _ = b.Close() }()
	check := func(c net.Conn) {
		t.Helper()
		_, _ = c.Write([]byte("x"))
		var buf [1]byte
		if _, err := io.ReadFull(c, buf[:]); err != nil {
			t.Fatal(err)
		}
	}
	check(a)
	check(b)
	cancel()
	check(b)
	if ssh.Connections.Load() != 2 {
		t.Fatalf("independent instances shared transport: %d", ssh.Connections.Load())
	}
}

func TestSSHForwardPendingHandshakeStopsOnCancellation(t *testing.T) {
	ssh := StartSSH(t, nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	cfg := ssh.Config()
	cfg.Addr = ln.Addr().String()
	addr, _, cancel, _ := startSSHForward(t, Config{Target: "application.test:10024", Upstream: cfg, DialTimeout: time.Hour})
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	var raw net.Conn
	select {
	case raw = <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("SSH handshake did not start")
	}
	defer func() { _ = raw.Close() }()
	cancel()
	_ = raw.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.ReadAll(raw); err != nil {
		t.Fatalf("handshake socket not released: %v", err)
	}
}

func assertForwardHTTPResponse(t *testing.T, client *http.Client, addr string) {
	t.Helper()
	resp, err := client.Get("http://" + addr + "/")
	if err != nil {
		t.Error(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "remote application" {
		t.Errorf("response=%q err=%v", body, err)
	}
}
