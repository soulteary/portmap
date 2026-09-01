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
	done := make(chan struct{})
	go func() {
		Relay(
			&IdleConn{Conn: client, Timeout: idleTimeout},
			&IdleConn{Conn: remote, Timeout: idleTimeout},
		)
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
