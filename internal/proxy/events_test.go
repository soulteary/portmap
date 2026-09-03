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
	"io"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/stats"
)

// startTestProxyWithEvents 启动一个挂载了 EventLog 的代理服务（手动 serveConn
// 循环，便于断言事件），返回其地址、事件日志与关闭函数。
func startTestProxyWithEvents(t *testing.T) (string, *stats.EventLog, func()) {
	t.Helper()
	events := stats.NewEventLog(100)
	srv := New("127.0.0.1:0")
	srv.Events = events
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	srv.listener = ln
	srv.Addr = ln.Addr().String()
	srv.DialTimeout = 5 * time.Second
	srv.dialer = NewDirectDialer(srv.DialTimeout, defaultKeepAlive)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.serveConn(conn)
		}
	}()

	return ln.Addr().String(), events, func() { _ = ln.Close() }
}

// TestProxyRecordsOpenCloseEvents 验证 HTTP 代理在连接开/关时写入事件，
// 且 close 事件携带解析出的目标地址与非零上/下行字节。
func TestProxyRecordsOpenCloseEvents(t *testing.T) {
	proxyAddr, events, stopProxy := startTestProxyWithEvents(t)
	defer stopProxy()
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	proxyURL, _ := url.Parse("http://" + proxyAddr)
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}
	resp, err := client.Get("http://" + backendAddr + "/")
	if err != nil {
		t.Fatalf("通过 HTTP 代理请求失败: %v", err)
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	var openEv, closeEv *stats.Event
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		openEv, closeEv = nil, nil
		for _, ev := range events.Snapshot() {
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
	if openEv.Proto != "http" {
		t.Errorf("open.Proto=%q, 期望 http", openEv.Proto)
	}
	if openEv.Client == "" {
		t.Errorf("open.Client 为空")
	}
	if closeEv == nil {
		t.Fatalf("未记录 close 事件: %+v", events.Snapshot())
	}
	if closeEv.Target != backendAddr {
		t.Errorf("close.Target=%q, 期望 %q", closeEv.Target, backendAddr)
	}
	if closeEv.UpBytes <= 0 {
		t.Errorf("close.UpBytes=%d, 期望 > 0", closeEv.UpBytes)
	}
	if closeEv.DownBytes <= 0 {
		t.Errorf("close.DownBytes=%d, 期望 > 0", closeEv.DownBytes)
	}
}
