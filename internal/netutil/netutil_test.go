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
