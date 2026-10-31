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

func startNativeProxy(t *testing.T, s *Server) (string, <-chan error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- s.ListenAndServe() }()
	t.Cleanup(func() { _ = s.Close() })
	limit := time.NewTimer(3 * time.Second)
	defer limit.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			t.Fatalf("startup: %v", err)
		case <-limit.C:
			t.Fatal("listener never registered")
		case <-tick.C:
			s.mu.Lock()
			ln := s.listener
			s.mu.Unlock()
			if ln != nil {
				return ln.Addr().String(), done
			}
		}
	}
}
func TestNativeProxyListenerLifecycle(t *testing.T) {
	s := &Server{Addr: "127.0.0.1:0", Logger: log.New(io.Discard, "", 0)}
	addr, done := startNativeProxy(t, s)
	if err := s.ListenAndServe(); !errors.Is(err, errServerAlreadyServing) {
		t.Fatalf("duplicate serve=%v", err)
	}
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := s.ListenAndServe(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("serve closed=%v", err)
	}
}
func TestNativeProxyConnectionLimit(t *testing.T) {
	s := New("127.0.0.1:0")
	s.MaxConns = 1
	s.Logger = log.New(io.Discard, "", 0)
	addr, done := startNativeProxy(t, s)
	c, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	until := time.Now().Add(time.Second)
	for s.Stats().ActiveConns() != 1 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if s.Stats().ActiveConns() != 1 {
		t.Fatal("first client not tracked")
	}
	second, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = second.Close() }()
	_ = second.SetReadDeadline(time.Now().Add(time.Second))
	_, err = second.Read(make([]byte, 1))
	if err == nil {
		t.Fatal("over-limit client accepted")
	}
	if got := s.Snapshot().RejectedConns; got != 1 {
		t.Fatalf("rejected=%d", got)
	}
	_ = s.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestProxyDialerInitialization(t *testing.T) {
	s := New("bad address")
	s.Logger = log.New(io.Discard, "", 0)
	s.Upstream = &UpstreamConfig{Scheme: "bad"}
	if err := s.ListenAndServe(); err == nil {
		t.Fatal("bad upstream accepted")
	}
	s.Upstream = &UpstreamConfig{Scheme: UpstreamSchemeHTTP, Addr: "127.0.0.1:1"}
	if err := s.ListenAndServe(); err == nil {
		t.Fatal("bad listener accepted")
	}
	s.closeDialer()
}

type failingListener struct{ calls int }

func (l *failingListener) Accept() (net.Conn, error) {
	l.calls++
	if l.calls == 1 {
		return nil, errors.New("transient accept")
	}
	return nil, net.ErrClosed
}
func (*failingListener) Close() error   { return nil }
func (*failingListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234} }
func TestProxyAcceptErrorRetry(t *testing.T) {
	s := New("")
	s.Logger = log.New(io.Discard, "", 0)
	l := &failingListener{}
	if err := s.acceptConnections(l); err != nil {
		t.Fatal(err)
	}
	if l.calls != 2 {
		t.Fatal(l.calls)
	}
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
func TestClosedProxyRejectsTransitions(t *testing.T) {
	s := New("")
	s.closing = true
	c := &memoryConn{}
	if s.setHandshakeDeadline(c, time.Now()) || s.completeHandshake(c) || s.trackConn(c) || s.trackRemote(c) || s.beginRelay(c, c) {
		t.Fatal("shutdown allowed a new transition")
	}
	if err := s.serverContext().Err(); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.stopAccepting(); err != nil {
		t.Fatal(err)
	}
}
func TestProxySelfTargetAddressEdges(t *testing.T) {
	s := New("")
	if s.isSelfTarget(context.Background(), "malformed") || s.isSelfTarget(context.Background(), "localhost:80") {
		t.Fatal("unbound server classified as self")
	}
	s.listener = &failingListener{}
	if !s.isSelfTarget(context.Background(), "localhost:1234") {
		t.Fatal("localhost missed")
	}
	if s.isSelfTarget(context.Background(), "invalid host:1234") {
		t.Fatal("invalid host matched")
	}
	if s.isSelfTarget(context.Background(), "192.0.2.1:1234") {
		t.Fatal("remote host matched")
	}
	c := &memoryConn{remote: &net.UDPAddr{}}
	if s.isSelfConn(c) {
		t.Fatal("non-TCP peer matched")
	}
}
func TestSOCKSRequestTruncationAndWriteErrors(t *testing.T) {
	valid := []byte{1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 80}
	for n := 0; n < len(valid); n++ {
		c := &memoryConn{}
		if _, err := (&Server{}).readSOCKSRequest(c, bufio.NewReader(bytes.NewReader(valid[:n]))); err == nil {
			t.Fatalf("truncated request %d accepted", n)
		}
	}
	c := &memoryConn{writeErr: io.ErrClosedPipe}
	if _, err := (&Server{}).readSOCKSRequest(c, bufio.NewReader(bytes.NewReader(valid))); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}
func TestSOCKSHandshakeCancellation(t *testing.T) {
	reader := func() *bufio.Reader {
		return bufio.NewReader(bytes.NewReader([]byte{1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 80}))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := New("")
	if err := s.handleSOCKS5WithReader(ctx, &memoryConn{}, reader()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.handleSOCKS5WithReader(context.Background(), &memoryConn{deadlineErr: io.ErrClosedPipe}, reader()); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestHTTPResponseReadAndWriteFailures(t *testing.T) {
	s := New("")
	req, _ := http.NewRequest("GET", "http://example.test/", nil)
	remote := &memoryConn{input: strings.NewReader("HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")}
	if err := s.relayHTTPResponses(remote, req, &countingWriter{w: &memoryConn{writeErr: io.ErrClosedPipe}}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := s.relayHTTPResponses(&memoryConn{input: strings.NewReader("bad")}, req, &countingWriter{w: io.Discard}); err == nil {
		t.Fatal("invalid response accepted")
	}
}

type dialFunc func(context.Context, string, string) (net.Conn, error)

func (f dialFunc) DialContext(ctx context.Context, n, a string) (net.Conn, error) {
	return f(ctx, n, a)
}
func TestHTTPForwardFailureBoundaries(t *testing.T) {
	for _, kind := range []string{"dial", "self", "closed", "write", "continue", "response"} {
		t.Run(kind, func(t *testing.T) {
			s := New("")
			s.Logger = log.New(io.Discard, "", 0)
			client := &memoryConn{}
			remote := &memoryConn{input: strings.NewReader("bad response")}
			s.dialer = dialFunc(func(_ context.Context, _, addr string) (net.Conn, error) {
				if addr != "example.test:80" {
					t.Errorf("target=%q", addr)
				}
				switch kind {
				case "dial":
					return nil, io.ErrClosedPipe
				case "self":
					s.listener = &failingListener{}
					remote.remote = s.listener.Addr()
				case "closed":
					s.closing = true
				}
				return remote, nil
			})
			switch kind {
			case "write":
				remote.writeErr = io.ErrClosedPipe
			case "continue":
				client.writeErr = io.ErrClosedPipe
			}
			req, _ := http.NewRequest("POST", "http://example.test/", strings.NewReader("payload"))
			req.Host = ""
			req.Header.Set("Expect", "100-continue")
			err := s.handlePlainHTTP(context.Background(), client, bufio.NewReader(strings.NewReader("")), req)
			if err == nil {
				t.Fatal("failed forwarding returned success")
			}
			if kind == "dial" && s.Snapshot().DialErrors != 1 {
				t.Fatal("dial failure not counted")
			}
			if kind != "dial" && !remote.closed {
				t.Fatal("remote connection leaked")
			}
		})
	}
}
func TestConnectFailureBoundaries(t *testing.T) {
	for _, kind := range []string{"dial", "self", "closed", "reply", "pre-self"} {
		t.Run(kind, func(t *testing.T) {
			s := New("")
			s.Logger = log.New(io.Discard, "", 0)
			client := &memoryConn{}
			remote := &memoryConn{}
			s.dialer = dialFunc(func(_ context.Context, _, addr string) (net.Conn, error) {
				if addr != "example.test:443" {
					t.Errorf("target=%q", addr)
				}
				switch kind {
				case "dial":
					return nil, io.ErrClosedPipe
				case "self":
					s.listener = &failingListener{}
					remote.remote = s.listener.Addr()
				case "closed":
					s.closing = true
				}
				return remote, nil
			})
			req := &http.Request{Host: "example.test"}
			if kind == "reply" {
				client.writeErr = io.ErrClosedPipe
			}
			if kind == "pre-self" {
				s.listener = &failingListener{}
				req.Host = "127.0.0.1:1234"
			}
			if err := s.handleConnect(context.Background(), client, bufio.NewReader(strings.NewReader("")), req); err == nil {
				t.Fatal("failed CONNECT returned success")
			}
			if kind != "dial" && kind != "pre-self" && !remote.closed {
				t.Fatal("remote connection leaked")
			}
		})
	}
}
func TestHTTPHandshakeAndMalformedResponses(t *testing.T) {
	raw := "GET http://example.test/ HTTP/1.1\r\nHost: example.test\r\n\r\n"
	s := New("")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.handleHTTP(ctx, &memoryConn{}, bufio.NewReader(strings.NewReader(raw))); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := s.handleHTTP(context.Background(), &memoryConn{deadlineErr: io.ErrClosedPipe}, bufio.NewReader(strings.NewReader(raw))); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := readProxyRequest(bufio.NewReader(strings.NewReader("invalid\r\n\r\n"))); err == nil {
		t.Fatal("malformed HTTP accepted")
	}
	for _, input := range []string{"invalid\r\n\r\n", strings.Repeat("x", maxProxyResponseHeaderBytes+1)} {
		if _, _, err := newProxyResponseReader(strings.NewReader(input)).read(nil); err == nil {
			t.Fatal("invalid response accepted")
		}
	}
	h := http.Header{"Upgrade": []string{"custom"}, "Connection": []string{"keep-alive"}}
	if isUpgradeRequest(&http.Request{Header: h}) {
		t.Fatal("unrequested upgrade accepted")
	}
	got := splitHeaderList(`token, "quoted\"value", end`)
	if len(got) != 3 {
		t.Fatal(got)
	}
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
func TestSOCKSPostDialFailures(t *testing.T) {
	for _, kind := range []string{"self", "closed"} {
		t.Run(kind, func(t *testing.T) {
			s := New("")
			remote := &memoryConn{}
			s.dialer = dialFunc(func(context.Context, string, string) (net.Conn, error) {
				if kind == "self" {
					s.listener = &failingListener{}
					remote.remote = s.listener.Addr()
				} else {
					s.closing = true
				}
				return remote, nil
			})
			reader := bufio.NewReader(bytes.NewReader([]byte{1, 0, 5, 1, 0, 1, 127, 0, 0, 1, 0, 80}))
			if err := s.handleSOCKS5WithReader(context.Background(), &memoryConn{}, reader); err == nil {
				t.Fatal("failed SOCKS transition succeeded")
			}
			if !remote.closed {
				t.Fatal("remote leaked")
			}
		})
	}
}

func TestDialerPublicationAfterShutdownClosesResources(t *testing.T) {
	s := New("")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	candidate := &closeTrackingDialer{}
	if err := s.installDialer(candidate); !errors.Is(err, net.ErrClosed) {
		t.Fatal(err)
	}
	if !candidate.closed {
		t.Fatal("late constructor leaked resources")
	}
	if s.dialer != nil {
		t.Fatal("dialer published after shutdown")
	}
	if err := s.installDialer(NewDirectDialer(time.Second, time.Second)); !errors.Is(err, net.ErrClosed) {
		t.Fatal(err)
	}
}

type closeTrackingDialer struct{ closed bool }

func (*closeTrackingDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	return nil, net.ErrClosed
}
func (d *closeTrackingDialer) Close() error { d.closed = true; return nil }
func TestConcurrentProxyInitializationAndClose(t *testing.T) {
	for i := 0; i < 100; i++ {
		s := New("")
		start := make(chan struct{})
		done := make(chan error, 2)
		go func() { <-start; done <- s.initializeDialer() }()
		go func() { <-start; done <- s.Close() }()
		close(start)
		for n := 0; n < 2; n++ {
			if err := <-done; err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatal(err)
			}
		}
		if err := s.initializeDialer(); !errors.Is(err, net.ErrClosed) {
			t.Fatal("closed server reinitialized")
		}
	}
}
