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

package forward

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// freePort 返回一个当前空闲的本地 TCP 端口。
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

// startEchoServer 启动一个 TCP 回显服务器，返回其地址与关闭函数。
func startEchoServer(t *testing.T) (addr string, closeFn func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen echo: %v", err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				_, _ = io.Copy(c, c)
			}(c)
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

// startServer 在指定端口后台启动转发服务，返回监听地址与用于等待退出的函数。
func startServer(t *testing.T, cfg Config) (listenAddr string, wait func()) {
	t.Helper()
	addr, _, wait := startServerRef(t, cfg)
	return addr, wait
}

// startServerRef 与 startServer 相同，但额外返回 *Server 引用，
// 便于测试观察 ActiveConns()/TotalConns() 等状态。
func startServerRef(t *testing.T, cfg Config) (listenAddr string, srv *Server, wait func()) {
	t.Helper()
	port := freePort(t)
	cfg.Listen = fmt.Sprintf("127.0.0.1:%d", port)
	srv = New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()

	// 等待端口就绪。
	waitPortReady(t, cfg.Listen, cfg.Network)

	return cfg.Listen, srv, func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("server exited with error: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Errorf("server did not exit within timeout")
		}
	}
}

func waitPortReady(t *testing.T, addr, network string) {
	t.Helper()
	if network == "" {
		network = "tcp"
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if network == "udp" {
			// UDP 监听立即可用，无需探测连接。
			time.Sleep(20 * time.Millisecond)
			return
		}
		c, err := net.Dial(network, addr)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("port %s not ready", addr)
}

// TestForward 验证经由转发器的数据能正确双向传输。
func TestForward(t *testing.T) {
	target, closeEcho := startEchoServer(t)
	defer closeEcho()

	addr, wait := startServer(t, Config{Target: target, ReuseAddr: true})
	defer wait()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()

	want := []byte("hello-socat")
	if _, err := conn.Write(want); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestContextCancelGracefulExit 验证 ctx 取消后 ListenAndServe 优雅退出。
func TestContextCancelGracefulExit(t *testing.T) {
	target, closeEcho := startEchoServer(t)
	defer closeEcho()

	port := freePort(t)
	listen := fmt.Sprintf("127.0.0.1:%d", port)
	srv := New(Config{Listen: listen, Target: target})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe(ctx) }()
	waitPortReady(t, listen, "tcp")

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error on graceful exit, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ListenAndServe did not return after ctx cancel")
	}
}

// TestUnreachableTarget 验证目标不可达时不会 panic，连接被正常关闭。
func TestUnreachableTarget(t *testing.T) {
	// 指向一个不可达端口。
	badPort := freePort(t)
	badTarget := fmt.Sprintf("127.0.0.1:%d", badPort)

	addr, wait := startServer(t, Config{Target: badTarget, DialTimeout: 500 * time.Millisecond})
	defer wait()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// 目标不可达，转发器应关闭连接：读取应返回 EOF/错误而非挂起。
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected connection to be closed on unreachable target")
	}
}

// TestConcurrentConnections 验证多连接并发回显均正确。
func TestConcurrentConnections(t *testing.T) {
	target, closeEcho := startEchoServer(t)
	defer closeEcho()

	addr, wait := startServer(t, Config{Target: target})
	defer wait()

	const n = 20
	var wg sync.WaitGroup
	var failures int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			defer func() { _ = conn.Close() }()
			msg := []byte(fmt.Sprintf("payload-%d", i))
			if _, err := conn.Write(msg); err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			got := make([]byte, len(msg))
			if _, err := io.ReadFull(conn, got); err != nil || string(got) != string(msg) {
				atomic.AddInt64(&failures, 1)
			}
		}(i)
	}
	wg.Wait()
	if failures != 0 {
		t.Fatalf("%d/%d concurrent connections failed", failures, n)
	}
}

// TestMaxConns 验证并发限流下连接依然能被处理（排队而非拒绝）。
func TestMaxConns(t *testing.T) {
	target, closeEcho := startEchoServer(t)
	defer closeEcho()

	addr, wait := startServer(t, Config{Target: target, MaxConns: 2})
	defer wait()

	const n = 6
	var wg sync.WaitGroup
	var failures int64
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			defer func() { _ = conn.Close() }()
			msg := []byte(fmt.Sprintf("m-%d", i))
			if _, err := conn.Write(msg); err != nil {
				atomic.AddInt64(&failures, 1)
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			got := make([]byte, len(msg))
			if _, err := io.ReadFull(conn, got); err != nil || string(got) != string(msg) {
				atomic.AddInt64(&failures, 1)
			}
		}(i)
	}
	wg.Wait()
	if failures != 0 {
		t.Fatalf("%d/%d connections failed under max-conns limit", failures, n)
	}
}

// TestIdleTimeout 验证空闲超时会断开无数据的连接。
func TestIdleTimeout(t *testing.T) {
	target, closeEcho := startEchoServer(t)
	defer closeEcho()

	addr, wait := startServer(t, Config{Target: target, IdleTimeout: 200 * time.Millisecond})
	defer wait()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// 不发送任何数据，等待超过空闲超时后连接应被断开。
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected connection to be closed after idle timeout")
	}
}

// TestUDPForward 验证 UDP 数据报双向转发。
func TestUDPForward(t *testing.T) {
	// UDP 回显目标。
	targetConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen udp target: %v", err)
	}
	defer func() { _ = targetConn.Close() }()
	go func() {
		buf := make([]byte, 2048)
		for {
			n, addr, err := targetConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = targetConn.WriteToUDP(buf[:n], addr)
		}
	}()

	addr, wait := startServer(t, Config{
		Network: "udp",
		Target:  targetConn.LocalAddr().String(),
	})
	defer wait()

	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("dial udp forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()

	want := []byte("udp-hello")
	if _, err := conn.Write(want); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(want))
	n, err := conn.Read(got)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got[:n]) != string(want) {
		t.Fatalf("got %q, want %q", got[:n], want)
	}
}

// startUDPEcho 启动一个 UDP 回显目标，返回其地址与关闭函数。
func startUDPEcho(t *testing.T) (addr string, closeFn func()) {
	t.Helper()
	targetConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen udp target: %v", err)
	}
	go func() {
		buf := make([]byte, 2048)
		for {
			n, from, err := targetConn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = targetConn.WriteToUDP(buf[:n], from)
		}
	}()
	return targetConn.LocalAddr().String(), func() { _ = targetConn.Close() }
}

// TestUDPSessionReuse 验证同一客户端连续多个数据报走同一会话且均被正确回显（竞态修复）。
func TestUDPSessionReuse(t *testing.T) {
	target, closeEcho := startUDPEcho(t)
	defer closeEcho()

	addr, wait := startServer(t, Config{Network: "udp", Target: target})
	defer wait()

	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("dial udp forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()

	const rounds = 10
	got := make([]byte, 2048)
	for i := 0; i < rounds; i++ {
		want := []byte(fmt.Sprintf("reuse-%d", i))
		if _, err := conn.Write(want); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(got)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if string(got[:n]) != string(want) {
			t.Fatalf("round %d: got %q, want %q", i, got[:n], want)
		}
	}
}

// TestUDPMaxConns 验证 MaxConns=1 时第二个不同客户端的会话被拒绝，第一个仍可用。
func TestUDPMaxConns(t *testing.T) {
	target, closeEcho := startUDPEcho(t)
	defer closeEcho()

	// idle 设大一些，确保测试期间第一个会话不会被回收。
	addr, wait := startServer(t, Config{Network: "udp", Target: target, MaxConns: 1, IdleTimeout: 30 * time.Second})
	defer wait()

	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// 第一个客户端：建立会话并确认回显可用。
	c1, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("dial c1: %v", err)
	}
	defer func() { _ = c1.Close() }()

	want1 := []byte("client-1")
	if _, err := c1.Write(want1); err != nil {
		t.Fatalf("c1 write: %v", err)
	}
	_ = c1.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, 2048)
	n, err := c1.Read(got)
	if err != nil {
		t.Fatalf("c1 read: %v", err)
	}
	if string(got[:n]) != string(want1) {
		t.Fatalf("c1 got %q, want %q", got[:n], want1)
	}

	// 第二个不同客户端：会话应被拒绝，读取应超时（收不到回包）。
	c2, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("dial c2: %v", err)
	}
	defer func() { _ = c2.Close() }()

	if _, err := c2.Write([]byte("client-2")); err != nil {
		t.Fatalf("c2 write: %v", err)
	}
	_ = c2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if _, err := c2.Read(got); err == nil {
		t.Fatal("expected second client to be refused (read timeout), but got a reply")
	}

	// 第一个客户端应仍然可用。
	want1b := []byte("client-1-again")
	if _, err := c1.Write(want1b); err != nil {
		t.Fatalf("c1 write again: %v", err)
	}
	_ = c1.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = c1.Read(got)
	if err != nil {
		t.Fatalf("c1 read again: %v", err)
	}
	if string(got[:n]) != string(want1b) {
		t.Fatalf("c1 got %q, want %q", got[:n], want1b)
	}
}

// closingUDPEcho 启动一个 UDP 目标：对每个来源仅回显首个数据报，
// 随后关闭底层 socket，使转发器后续对该目标的写入失败，从而触发
// serveUDP 的重建路径。restart 用于在同一地址上重新拉起回显。
type closingUDPEcho struct {
	addr string
	conn *net.UDPConn
}

func startClosingUDPEcho(t *testing.T) *closingUDPEcho {
	t.Helper()
	c, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("listen udp target: %v", err)
	}
	e := &closingUDPEcho{addr: c.LocalAddr().String(), conn: c}
	go e.serve(c)
	return e
}

func (e *closingUDPEcho) serve(c *net.UDPConn) {
	buf := make([]byte, 2048)
	for {
		n, from, err := c.ReadFromUDP(buf)
		if err != nil {
			return
		}
		_, _ = c.WriteToUDP(buf[:n], from)
	}
}

// restart 在原地址上重新监听，使目标再次可用。
func (e *closingUDPEcho) restart(t *testing.T) {
	t.Helper()
	raddr, err := net.ResolveUDPAddr("udp", e.addr)
	if err != nil {
		t.Fatalf("resolve target: %v", err)
	}
	c, err := net.ListenUDP("udp", raddr)
	if err != nil {
		t.Fatalf("re-listen udp target: %v", err)
	}
	e.conn = c
	go e.serve(c)
}

func (e *closingUDPEcho) close() { _ = e.conn.Close() }

// TestUDPRebuildUnderMaxConns 回归测试 UDP 重建路径的限流竞态：
// MaxConns=1 下，同一客户端在一次目标写失败后应仍能重建会话并正常回显，
// 而不会因旧 relay 尚未归还 sem 被误判为限流拒绝。
func TestUDPRebuildUnderMaxConns(t *testing.T) {
	echo := startClosingUDPEcho(t)
	defer echo.close()

	addr, wait := startServer(t, Config{Network: "udp", Target: echo.addr, MaxConns: 1, IdleTimeout: 30 * time.Second})
	defer wait()

	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("dial udp forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// 第一轮：建立会话并确认回显可用。
	want := []byte("round-1")
	if _, err := conn.Write(want); err != nil {
		t.Fatalf("write 1: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, 2048)
	n, err := conn.Read(got)
	if err != nil {
		t.Fatalf("read 1: %v", err)
	}
	if string(got[:n]) != string(want) {
		t.Fatalf("round 1: got %q, want %q", got[:n], want)
	}

	// 关闭目标使转发器后续写入失败，再在原地址重启目标。
	echo.close()
	// 给 relay 一点时间感知目标关闭（读到错误后退出、归还 sem）。
	time.Sleep(100 * time.Millisecond)
	echo.restart(t)

	// 第二轮：同一客户端应能在写失败后重建会话并正常回显。
	// 由于目标已连接过的 socket 可能收到 ICMP 不可达，首次写会失败并触发重建。
	var lastErr error
	for i := 0; i < 5; i++ {
		want2 := []byte(fmt.Sprintf("round-2-%d", i))
		if _, err := conn.Write(want2); err != nil {
			t.Fatalf("write 2: %v", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, err = conn.Read(got)
		if err != nil {
			lastErr = err
			continue
		}
		if string(got[:n]) == string(want2) {
			return // 成功重建并回显
		}
	}
	t.Fatalf("client could not re-establish session under MaxConns=1 (last err: %v)", lastErr)
}

// TestUDPIdleRecycle 验证 UDP 会话在空闲超时后被回收（ActiveConns 归零）。
func TestUDPIdleRecycle(t *testing.T) {
	target, closeEcho := startUDPEcho(t)
	defer closeEcho()

	addr, srv, wait := startServerRef(t, Config{Network: "udp", Target: target, IdleTimeout: 200 * time.Millisecond})
	defer wait()

	raddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		t.Fatalf("dial udp forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()

	want := []byte("idle-me")
	if _, err := conn.Write(want); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, 2048)
	if _, err := conn.Read(got); err != nil {
		t.Fatalf("read: %v", err)
	}
	if srv.ActiveConns() != 1 {
		t.Fatalf("expected 1 active session, got %d", srv.ActiveConns())
	}

	// 空闲超过 idle 后会话应被回收，ActiveConns 归零。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.ActiveConns() == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("expected session to be recycled after idle, ActiveConns=%d", srv.ActiveConns())
}

// TestIdleTimeoutClosesBothDirections 验证一个方向空闲触发超时后整条 TCP 连接关闭。
func TestIdleTimeoutClosesBothDirections(t *testing.T) {
	// 目标：只在收到数据时回一次，之后保持静默，制造一个方向空闲。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen target: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				buf := make([]byte, 1024)
				// 读取一次并回显，然后不再发送，等待被空闲超时关闭。
				n, err := c.Read(buf)
				if err != nil {
					return
				}
				_, _ = c.Write(buf[:n])
				// 阻塞读取直到连接被关闭。
				_, _ = c.Read(buf)
			}(c)
		}
	}()

	addr, wait := startServer(t, Config{Target: ln.Addr().String(), IdleTimeout: 200 * time.Millisecond})
	defer wait()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// 发送一次数据并读回，之后两个方向都空闲。
	want := []byte("ping")
	if _, err := conn.Write(want); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read echo: %v", err)
	}

	// 空闲超时后整条连接应被关闭：后续读取应返回 EOF/错误。
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("expected connection to be closed after idle timeout in one direction")
	}
}
