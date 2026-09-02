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
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

// timeoutError 是一个 net.Error，Timeout() 返回 true，用于覆盖 isNormalClose
// 的空闲超时分支。
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// nonTimeoutNetError 是一个 net.Error，但 Timeout() 返回 false，用于覆盖
// isNormalClose 中「是 net.Error 但非超时」的否定分支。
type nonTimeoutNetError struct{}

func (nonTimeoutNetError) Error() string   { return "some net error" }
func (nonTimeoutNetError) Timeout() bool   { return false }
func (nonTimeoutNetError) Temporary() bool { return false }

// TestIsNormalClose 覆盖 isNormalClose 的全部分支：nil、EOF、ErrClosed、
// Canceled、超时 net.Error（正常），以及非超时错误与普通错误（异常）。
func TestIsNormalClose(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, true},
		{"EOF", io.EOF, true},
		{"包装的 EOF", fmt.Errorf("read: %w", io.EOF), true},
		{"ErrClosed", net.ErrClosed, true},
		{"Canceled", context.Canceled, true},
		{"超时 net.Error", timeoutError{}, true},
		{"非超时 net.Error", nonTimeoutNetError{}, false},
		{"普通错误", errors.New("boom"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isNormalClose(c.err); got != c.want {
				t.Fatalf("isNormalClose(%v)=%v，期望 %v", c.err, got, c.want)
			}
		})
	}
}

// TestTotalConnsIncrements 验证 TotalConns 随连接累计递增（与 ActiveConns 区分：
// 累计值在连接关闭后仍保留）。
func TestTotalConnsIncrements(t *testing.T) {
	target, closeEcho := startEchoServer(t)
	defer closeEcho()

	addr, srv, wait := startServerRef(t, Config{Target: target, ReuseAddr: true})
	defer wait()

	if srv.TotalConns() != 0 {
		t.Fatalf("初始 TotalConns 应为 0，实际 %d", srv.TotalConns())
	}

	const rounds = 3
	for i := 0; i < rounds; i++ {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			t.Fatalf("拨号失败: %v", err)
		}
		if _, err := conn.Write([]byte("x")); err != nil {
			t.Fatalf("写入失败: %v", err)
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 1)
		if _, err := io.ReadFull(conn, buf); err != nil {
			t.Fatalf("读回失败: %v", err)
		}
		_ = conn.Close()
	}

	// TotalConns 在 handle 中于拨号成功后自增；轮询等待其达到期望值。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.TotalConns() >= rounds {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := srv.TotalConns(); got < rounds {
		t.Fatalf("TotalConns=%d，期望至少 %d", got, rounds)
	}
}
