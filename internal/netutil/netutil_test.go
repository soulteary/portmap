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

package netutil

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

func TestIdleConnWriteTimeout(t *testing.T) {
	writer, reader := net.Pipe()
	defer func() { _ = writer.Close() }()
	defer func() { _ = reader.Close() }()

	conn := &IdleConn{Conn: writer, Timeout: 30 * time.Millisecond}
	started := time.Now()
	_, err := conn.Write([]byte("blocked"))
	if err == nil {
		t.Fatal("无人读取时 Write 应在空闲超时后失败")
	}
	if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
		t.Fatalf("Write 返回 %v，期望超时错误", err)
	}
	if elapsed := time.Since(started); elapsed < 20*time.Millisecond {
		t.Fatalf("Write 过早返回: %s", elapsed)
	}
}

func TestIdleConnOneWayTrafficKeepsReverseDirectionAlive(t *testing.T) {
	client, clientPeer := net.Pipe()
	remote, remotePeer := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = clientPeer.Close() }()
	defer func() { _ = remote.Close() }()
	defer func() { _ = remotePeer.Close() }()

	const idleTimeout = 80 * time.Millisecond
	clientIdle := &IdleConn{Conn: client, Timeout: idleTimeout}
	remoteIdle := &IdleConn{Conn: remote, Timeout: idleTimeout}
	ShareIdleTimeout(clientIdle, remoteIdle, idleTimeout)
	done := make(chan struct{})
	go func() {
		Relay(clientIdle, remoteIdle)
		close(done)
	}()

	// Keep the tunnel active in only the client-to-remote direction for much
	// longer than the idle timeout.
	until := time.Now().Add(4 * idleTimeout)
	for time.Now().Before(until) {
		if err := clientPeer.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := clientPeer.Write([]byte{'x'}); err != nil {
			t.Fatalf("持续单向写入失败: %v", err)
		}
		if err := remotePeer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := io.ReadFull(remotePeer, make([]byte, 1)); err != nil {
			t.Fatalf("持续单向读取失败: %v", err)
		}
		time.Sleep(idleTimeout / 4)
	}

	// Reverse traffic must still work because the tunnel itself was active.
	reverseWrite := make(chan error, 1)
	go func() {
		_, err := remotePeer.Write([]byte("pong"))
		reverseWrite <- err
	}()
	if err := clientPeer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len("pong"))
	if _, err := io.ReadFull(clientPeer, got); err != nil {
		t.Fatalf("单向传输后的反向读取失败: %v", err)
	}
	if string(got) != "pong" {
		t.Fatalf("反向读取=%q，期望 pong", got)
	}
	if err := <-reverseWrite; err != nil {
		t.Fatalf("反向写入失败: %v", err)
	}

	_ = clientPeer.Close()
	_ = remotePeer.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Relay 未在连接关闭后退出")
	}
}

// TestCopyAndCloseWriteHalfClosesTCP 用真实 TCP 连接验证 CopyAndCloseWrite
// 拷贝完成后半关闭写方向，使对端读到 EOF。
func TestCopyAndCloseWriteHalfClosesTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	defer func() { _ = ln.Close() }()

	accepted := make(chan net.Conn, 1)
	go func() {
		c, aerr := ln.Accept()
		if aerr == nil {
			accepted <- c
		}
	}()

	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("拨号失败: %v", err)
	}
	defer func() { _ = client.Close() }()

	var server net.Conn
	select {
	case server = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("未接受连接")
	}
	defer func() { _ = server.Close() }()

	payload := "hello-half-close"
	src := strings.NewReader(payload)
	n, err := CopyAndCloseWrite(client, src)
	if err != nil {
		t.Fatalf("CopyAndCloseWrite 返回错误: %v", err)
	}
	if int(n) != len(payload) {
		t.Fatalf("拷贝字节数=%d，期望 %d", n, len(payload))
	}

	// 服务端应能读到全部数据并随后读到 EOF（因写方向已半关闭）。
	got, err := io.ReadAll(server)
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if string(got) != payload {
		t.Fatalf("读取=%q，期望 %q", got, payload)
	}
}

// TestCopyAndCloseWriteFallbackNonTCP 验证连接不支持 CloseWrite 时走
// SetReadDeadline 回退路径（net.Pipe 不实现 halfCloser）。
func TestCopyAndCloseWriteFallbackNonTCP(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	// 后台把服务端写入的数据读走，避免 Copy 因无人读取而阻塞。
	read := make(chan string, 1)
	go func() {
		buf := make([]byte, 16)
		n, _ := server.Read(buf)
		read <- string(buf[:n])
	}()

	n, err := CopyAndCloseWrite(client, strings.NewReader("data"))
	if err != nil {
		t.Fatalf("CopyAndCloseWrite 返回错误: %v", err)
	}
	if int(n) != 4 {
		t.Fatalf("拷贝字节数=%d，期望 4", n)
	}
	select {
	case got := <-read:
		if got != "data" {
			t.Fatalf("服务端读取=%q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("服务端未读到数据")
	}
}

// TestIdleConnCloseWriteFallback 验证 IdleConn.CloseWrite 在底层不支持半关闭
// 时回退为设置立即读截止时间（不返回错误即视为回退成功）。
func TestIdleConnCloseWriteFallback(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = server.Close() }()
	defer func() { _ = client.Close() }()

	ic := &IdleConn{Conn: client}
	if err := ic.CloseWrite(); err != nil {
		t.Fatalf("非 TCP 连接 CloseWrite 回退应无错误: %v", err)
	}
}

// TestIdleConnCloseWriteTCP 验证 IdleConn 包装真实 TCP 连接时 CloseWrite 生效。
func TestIdleConnCloseWriteTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	defer func() { _ = ln.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, aerr := ln.Accept()
		if aerr == nil {
			accepted <- c
		}
	}()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("拨号失败: %v", err)
	}
	defer func() { _ = client.Close() }()
	var server net.Conn
	select {
	case server = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("未接受连接")
	}
	defer func() { _ = server.Close() }()

	ic := &IdleConn{Conn: client}
	if err := ic.CloseWrite(); err != nil {
		t.Fatalf("TCP 连接 CloseWrite 应成功: %v", err)
	}
	// 半关闭后服务端读到 EOF。
	if _, err := io.ReadAll(server); err != nil {
		t.Fatalf("半关闭后读取应正常返回 EOF: %v", err)
	}
}

// TestRelayReaderPropagatesEOFBothDirections 验证 RelayReader 在两端都能传播
// EOF：一端关闭后另一端也随之退出。
func TestRelayReaderPropagatesEOFBothDirections(t *testing.T) {
	a1, a2 := net.Pipe()
	b1, b2 := net.Pipe()
	defer func() { _ = a2.Close() }()
	defer func() { _ = b2.Close() }()

	done := make(chan struct{})
	go func() {
		Relay(a1, b1)
		close(done)
	}()

	// 从 a2 写入 -> 经 relay -> b2 读出。
	go func() { _, _ = a2.Write([]byte("ping")) }()
	buf := make([]byte, 4)
	_ = b2.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.ReadFull(b2, buf); err != nil {
		t.Fatalf("正向读取失败: %v", err)
	}
	if string(buf) != "ping" {
		t.Fatalf("正向读取=%q", buf)
	}

	// 关闭两端触发 relay 退出。
	_ = a2.Close()
	_ = b2.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Relay 未退出")
	}
}
