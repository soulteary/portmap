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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

type contextBlockingDialer struct {
	started chan struct{}
	stopped chan error
}

func (d *contextBlockingDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	close(d.started)
	<-ctx.Done()
	err := ctx.Err()
	d.stopped <- err
	return nil, err
}

type stubbornDialer struct {
	started chan struct{}
	release chan struct{}
}

// startHeldOpenConnectRelay creates an established CONNECT tunnel whose target
// keeps its read side open after receiving FIN. This reproduces the case where
// closing only the inbound connection leaves the reverse relay blocked.
func startHeldOpenConnectRelay(t *testing.T) (*Server, net.Conn, net.Conn, <-chan struct{}) {
	t.Helper()
	targetListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("目标监听失败: %v", err)
	}
	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, acceptErrValue := targetListener.Accept()
		if acceptErrValue != nil {
			acceptErr <- acceptErrValue
			return
		}
		accepted <- conn
		// Read until the proxy half-closes its write side, but deliberately keep
		// the connection open so the opposite proxy copy remains blocked.
		_, _ = io.Copy(io.Discard, conn)
	}()

	srv := New("127.0.0.1:0")
	srv.HandshakeTimeout = 0
	srv.IdleTimeout = 0
	srv.dialer = NewDirectDialer(time.Second, defaultKeepAlive)
	serverConn, clientConn := net.Pipe()
	if !srv.trackConn(serverConn) {
		t.Fatal("连接注册失败")
	}
	handlerDone := make(chan struct{})
	go func() {
		defer srv.untrackConn(serverConn)
		srv.serveConn(serverConn)
		close(handlerDone)
	}()

	if _, err := fmt.Fprintf(clientConn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", targetListener.Addr(), targetListener.Addr()); err != nil {
		t.Fatalf("发送 CONNECT 请求失败: %v", err)
	}
	reader := bufio.NewReader(clientConn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("读取 CONNECT 响应失败: %v", err)
	}
	if status != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("CONNECT 状态行=%q", status)
	}
	if line, err := reader.ReadString('\n'); err != nil || line != "\r\n" {
		t.Fatalf("读取 CONNECT 响应结尾=%q, %v", line, err)
	}

	var targetConn net.Conn
	select {
	case targetConn = <-accepted:
	case err := <-acceptErr:
		t.Fatalf("接受目标连接失败: %v", err)
	case <-time.After(time.Second):
		t.Fatal("代理未连接目标")
	}
	deadline := time.Now().Add(time.Second)
	for {
		srv.mu.Lock()
		_, handshaking := srv.handshakes[serverConn]
		srv.mu.Unlock()
		if !handshaking {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("CONNECT 成功后仍处于握手阶段")
		}
		time.Sleep(time.Millisecond)
	}
	_ = targetListener.Close()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = targetConn.Close()
	})
	return srv, clientConn, targetConn, handlerDone
}

func (d *stubbornDialer) DialContext(context.Context, string, string) (net.Conn, error) {
	close(d.started)
	<-d.release
	return nil, context.Canceled
}

func TestHTTPProxyRejectsSelfTarget(t *testing.T) {
	proxyAddr, stopProxy := startTestProxy(t)
	defer stopProxy()

	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second)
	if err != nil {
		t.Fatalf("连接代理失败: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := fmt.Fprintf(conn, "GET http://%s/ HTTP/1.1\r\nHost: %s\r\n\r\n", proxyAddr, proxyAddr); err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("读取代理响应失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusLoopDetected {
		t.Fatalf("自回连请求状态码=%d，期望 %d", resp.StatusCode, http.StatusLoopDetected)
	}
}

func TestSOCKS5ProxyRejectsSelfTarget(t *testing.T) {
	proxyAddr, stopProxy := startTestProxy(t)
	defer stopProxy()

	if conn, err := socks5Dial(proxyAddr, proxyAddr); err == nil {
		_ = conn.Close()
		t.Fatal("SOCKS5 自回连请求应被拒绝")
	}
}

func TestHandshakeTimeoutClosesSlowClient(t *testing.T) {
	srv := New("127.0.0.1:0")
	srv.HandshakeTimeout = 40 * time.Millisecond
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()

	done := make(chan struct{})
	go func() {
		srv.serveConn(serverConn)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("未完成握手的连接没有在超时后关闭")
	}
}

func TestHandshakeTimeoutDoesNotCapOutboundDial(t *testing.T) {
	dialer := &contextBlockingDialer{
		started: make(chan struct{}),
		stopped: make(chan error, 1),
	}
	srv := New("127.0.0.1:0")
	srv.HandshakeTimeout = 40 * time.Millisecond
	srv.DialTimeout = time.Second
	srv.dialer = dialer
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()

	done := make(chan struct{})
	go func() {
		srv.serveConn(serverConn)
		close(done)
	}()
	if _, err := io.WriteString(clientConn, "CONNECT example.test:443 HTTP/1.1\r\nHost: example.test:443\r\n\r\n"); err != nil {
		t.Fatalf("发送 CONNECT 请求失败: %v", err)
	}

	select {
	case <-dialer.started:
	case <-time.After(time.Second):
		t.Fatal("出站拨号没有开始")
	}
	select {
	case err := <-dialer.stopped:
		t.Fatalf("出站拨号被握手超时提前取消: %v", err)
	case <-time.After(3 * srv.HandshakeTimeout):
	}

	go func() { _, _ = io.Copy(io.Discard, clientConn) }()
	_ = srv.stopAccepting()
	select {
	case err := <-dialer.stopped:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("服务关闭后的拨号上下文返回 %v，期望 canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("服务关闭没有取消出站拨号")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("取消拨号后连接处理器没有退出")
	}
}

func TestHTTPProxyRejectsOversizedRequestHeaders(t *testing.T) {
	srv := New("127.0.0.1:0")
	srv.HandshakeTimeout = time.Second
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()

	done := make(chan struct{})
	go func() {
		srv.serveConn(serverConn)
		close(done)
	}()
	writeDone := make(chan error, 1)
	go func() {
		_, err := io.WriteString(clientConn,
			"GET http://example.test/ HTTP/1.1\r\nX-Large: "+
				strings.Repeat("a", maxProxyRequestHeaderBytes)+"\r\n\r\n")
		writeDone <- err
	}()

	resp, err := http.ReadResponse(bufio.NewReader(clientConn), nil)
	if err != nil {
		t.Fatalf("读取超大请求头响应失败: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusRequestHeaderFieldsTooLarge {
		t.Fatalf("状态码=%d，期望 %d", resp.StatusCode, http.StatusRequestHeaderFieldsTooLarge)
	}
	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("请求写入未在拒绝后退出")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("超大请求头处理器未退出")
	}
}

func TestConnectionLimit(t *testing.T) {
	srv := New("127.0.0.1:0")
	srv.MaxConns = 1
	first, firstPeer := net.Pipe()
	defer func() { _ = firstPeer.Close() }()
	second, secondPeer := net.Pipe()
	defer func() { _ = secondPeer.Close() }()

	if !srv.trackConn(first) {
		t.Fatal("第一个连接应被接纳")
	}
	if srv.trackConn(second) {
		t.Fatal("达到连接上限后应拒绝新连接")
	}
	srv.untrackConn(first)
	_ = first.Close()

	if !srv.trackConn(second) {
		t.Fatal("连接释放后应允许新连接")
	}
	srv.untrackConn(second)
	_ = second.Close()
}

func TestRejectsPublicListenByDefault(t *testing.T) {
	srv := New("0.0.0.0:0")
	if err := srv.ListenAndServe(); err == nil {
		t.Fatal("默认应拒绝监听非回环地址")
	}
}

func TestShutdownInterruptsHandshakeBeforeDeadline(t *testing.T) {
	srv := New("127.0.0.1:0")
	srv.HandshakeTimeout = 0
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	if !srv.trackConn(serverConn) {
		t.Fatal("连接注册失败")
	}
	go func() {
		defer srv.untrackConn(serverConn)
		srv.serveConn(serverConn)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown 返回错误: %v", err)
	}
}

func TestShutdownInterruptsHandshakeWithoutTimeout(t *testing.T) {
	srv := New("127.0.0.1:0")
	srv.HandshakeTimeout = 0
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	if !srv.trackConn(serverConn) {
		t.Fatal("连接注册失败")
	}
	handlerDone := make(chan struct{})
	go func() {
		defer srv.untrackConn(serverConn)
		srv.serveConn(serverConn)
		close(handlerDone)
	}()

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- srv.Shutdown(context.Background()) }()
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown 返回错误: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown 未中断无超时的握手读取")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("握手读取取消后处理器未退出")
	}
}

func TestShutdownCancelsOutboundDial(t *testing.T) {
	dialer := &contextBlockingDialer{
		started: make(chan struct{}),
		stopped: make(chan error, 1),
	}
	srv := New("127.0.0.1:0")
	srv.HandshakeTimeout = 0
	srv.DialTimeout = time.Hour
	srv.dialer = dialer
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	if !srv.trackConn(serverConn) {
		t.Fatal("连接注册失败")
	}

	done := make(chan struct{})
	go func() {
		defer srv.untrackConn(serverConn)
		srv.serveConn(serverConn)
		close(done)
	}()
	if _, err := io.WriteString(clientConn, "CONNECT example.test:443 HTTP/1.1\r\nHost: example.test:443\r\n\r\n"); err != nil {
		t.Fatalf("发送 CONNECT 请求失败: %v", err)
	}
	select {
	case <-dialer.started:
	case <-time.After(time.Second):
		t.Fatal("出站拨号没有开始")
	}
	go func() { _, _ = io.Copy(io.Discard, clientConn) }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown 返回错误: %v", err)
	}
	select {
	case err := <-dialer.stopped:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("拨号上下文返回 %v，期望 canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown 没有取消出站拨号")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("取消拨号后连接处理器没有退出")
	}
}

func TestShutdownReturnsAtDeadlineWhenDialerIgnoresContext(t *testing.T) {
	dialer := &stubbornDialer{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	srv := New("127.0.0.1:0")
	srv.HandshakeTimeout = 0
	srv.DialTimeout = time.Hour
	srv.dialer = dialer
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()
	if !srv.trackConn(serverConn) {
		t.Fatal("连接注册失败")
	}

	done := make(chan struct{})
	go func() {
		defer srv.untrackConn(serverConn)
		srv.serveConn(serverConn)
		close(done)
	}()
	if _, err := io.WriteString(clientConn, "CONNECT example.test:443 HTTP/1.1\r\nHost: example.test:443\r\n\r\n"); err != nil {
		t.Fatalf("发送 CONNECT 请求失败: %v", err)
	}
	select {
	case <-dialer.started:
	case <-time.After(time.Second):
		t.Fatal("出站拨号没有开始")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	started := time.Now()
	err := srv.Shutdown(ctx)
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown 返回 %v，期望 deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > 300*time.Millisecond {
		t.Fatalf("Shutdown 超过截止时间后仍等待处理器: %s", elapsed)
	}

	close(dialer.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("释放拨号器后连接处理器没有退出")
	}
}

func TestCloseClosesOutboundRelay(t *testing.T) {
	srv, _, targetConn, handlerDone := startHeldOpenConnectRelay(t)
	done := make(chan error, 1)
	go func() { done <- srv.Close() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close 返回错误: %v", err)
		}
	case <-time.After(time.Second):
		_ = targetConn.Close()
		<-done
		t.Fatal("Close 未关闭出站连接，仍在等待反向中继")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("Close 返回后连接处理器仍未退出")
	}
}

func TestShutdownDeadlineClosesOutboundRelay(t *testing.T) {
	srv, _, _, handlerDone := startHeldOpenConnectRelay(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := srv.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown 返回 %v，期望 deadline exceeded", err)
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("Shutdown 截止后未关闭出站连接")
	}
}

func TestShutdownAllowsEstablishedRelayToDrain(t *testing.T) {
	srv, clientConn, targetConn, handlerDone := startHeldOpenConnectRelay(t)
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- srv.Shutdown(context.Background()) }()

	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown 在已建立中继结束前返回: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	_ = clientConn.Close()
	_ = targetConn.Close()
	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown 返回错误: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("已建立中继结束后 Shutdown 未返回")
	}
	select {
	case <-handlerDone:
	case <-time.After(time.Second):
		t.Fatal("已建立中继结束后处理器未退出")
	}
}
