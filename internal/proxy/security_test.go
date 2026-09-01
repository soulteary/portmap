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
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

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

func TestShutdownForcesConnectionsAtDeadline(t *testing.T) {
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
	if err := srv.Shutdown(ctx); err != context.DeadlineExceeded {
		t.Fatalf("Shutdown 返回 %v，期望 context deadline exceeded", err)
	}
}
