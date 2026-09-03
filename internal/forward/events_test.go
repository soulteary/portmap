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
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/stats"
)

// TestForwardRecordsOpenCloseEvents 验证 TCP 转发在连接开/关时向 EventLog
// 写入带有正确 client/target 的 "open" 与 "close" 事件。
func TestForwardRecordsOpenCloseEvents(t *testing.T) {
	target, closeEcho := startEchoServer(t)
	defer closeEcho()

	events := stats.NewEventLog(100)
	addr, wait := startServer(t, Config{Target: target, ReuseAddr: true, Events: events})
	defer wait()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}

	want := []byte("event-hello")
	if _, err := conn.Write(want); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	got := make([]byte, len(want))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatalf("read: %v", err)
	}
	localAddr := conn.LocalAddr().String()
	// 关闭客户端连接，促使 relay 结束并写入 close 事件。
	_ = conn.Close()

	// 等待与本连接（按 client 地址匹配）对应的 open 与 close 事件出现。
	// 注意：waitPortReady 的探测连接也会产生一对 open/close 事件，因此必须
	// 按 client 地址过滤，不能简单取最后一条。
	var openEv, closeEv *stats.Event
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		openEv, closeEv = nil, nil
		for _, ev := range events.Snapshot() {
			if ev.Client != localAddr {
				continue
			}
			e := ev
			switch e.Kind {
			case "open":
				openEv = &e
			case "close":
				closeEv = &e
			}
		}
		if openEv != nil && closeEv != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if openEv == nil {
		t.Fatalf("未记录 open 事件: %+v", events.Snapshot())
	}
	if closeEv == nil {
		t.Fatalf("未记录 close 事件: %+v", events.Snapshot())
	}

	if openEv.Proto != "tcp" {
		t.Errorf("open.Proto=%q, 期望 tcp", openEv.Proto)
	}
	if openEv.Client != localAddr {
		t.Errorf("open.Client=%q, 期望 %q", openEv.Client, localAddr)
	}
	if openEv.Target != target {
		t.Errorf("open.Target=%q, 期望 %q", openEv.Target, target)
	}
	if openEv.Time == "" {
		t.Errorf("open.Time 为空")
	}
	if closeEv.Client != localAddr {
		t.Errorf("close.Client=%q, 期望 %q", closeEv.Client, localAddr)
	}
	if closeEv.Target != target {
		t.Errorf("close.Target=%q, 期望 %q", closeEv.Target, target)
	}
	if closeEv.UpBytes < int64(len(want)) {
		t.Errorf("close.UpBytes=%d, 期望 >= %d", closeEv.UpBytes, len(want))
	}
	if closeEv.ConnID != openEv.ConnID {
		t.Errorf("open/close ConnID 不一致: %d vs %d", openEv.ConnID, closeEv.ConnID)
	}
}

// TestForwardRecordsDialError 验证目标不可达时写入 "dial-error" 事件。
func TestForwardRecordsDialError(t *testing.T) {
	badPort := freePort(t)
	badTarget := fmt.Sprintf("127.0.0.1:%d", badPort)

	events := stats.NewEventLog(100)
	addr, wait := startServer(t, Config{Target: badTarget, DialTimeout: 500 * time.Millisecond, Events: events})
	defer wait()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial forwarder: %v", err)
	}
	defer func() { _ = conn.Close() }()
	// 触发拨号：读取会因目标不可达而返回错误/EOF。
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Read(make([]byte, 1))

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, ev := range events.Snapshot() {
			if ev.Kind == "dial-error" {
				if ev.Target != badTarget {
					t.Errorf("dial-error.Target=%q, 期望 %q", ev.Target, badTarget)
				}
				if ev.Proto != "tcp" {
					t.Errorf("dial-error.Proto=%q, 期望 tcp", ev.Proto)
				}
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("未记录 dial-error 事件: %+v", events.Snapshot())
}
