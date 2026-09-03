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

package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/stats"
)

// fakeProvider 是一个实现 stats.Provider 的测试桩，返回固定快照。
type fakeProvider struct{ snap stats.Snapshot }

func (f fakeProvider) Snapshot() stats.Snapshot { return f.snap }

// TestServerRefusesNonLoopback 验证：以公网地址且 AllowPublic=false 构造 Server
// 时，ListenAndServe 应返回错误（拒绝监听）。
func TestServerRefusesNonLoopback(t *testing.T) {
	s := &Server{Addr: "0.0.0.0:0", Providers: []stats.Provider{stats.New()}}
	err := s.ListenAndServe(context.Background())
	if err == nil {
		t.Fatal("expected error for non-loopback address without AllowPublic")
	}
}

// TestServerEmptyAddrDisabled 验证：空 Addr 视为未启用，ListenAndServe 直接返回 nil。
func TestServerEmptyAddrDisabled(t *testing.T) {
	s := &Server{Addr: "", Providers: []stats.Provider{stats.New()}}
	if err := s.ListenAndServe(context.Background()); err != nil {
		t.Errorf("empty addr should be a no-op, got %v", err)
	}
}

// TestHandlerStatsJSON 验证 /api/stats 返回 200 与合法 JSON，且字段与聚合快照一致。
func TestHandlerStatsJSON(t *testing.T) {
	s := &Server{Providers: []stats.Provider{fakeProvider{stats.Snapshot{
		ActiveConns: 2, TotalConns: 7, RejectedConns: 1, DialErrors: 3,
		UpBytes: 1024, DownBytes: 2048, Uptime: 42 * time.Second,
	}}}}

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type=%q, want application/json", ct)
	}
	var payload struct {
		ActiveConns   int64   `json:"active_conns"`
		TotalConns    int64   `json:"total_conns"`
		RejectedConns int64   `json:"rejected_conns"`
		DialErrors    int64   `json:"dial_errors"`
		UpBytes       int64   `json:"up_bytes"`
		DownBytes     int64   `json:"down_bytes"`
		UptimeSeconds float64 `json:"uptime_seconds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("invalid JSON: %v (%s)", err, rec.Body.String())
	}
	if payload.ActiveConns != 2 || payload.TotalConns != 7 || payload.RejectedConns != 1 ||
		payload.DialErrors != 3 || payload.UpBytes != 1024 || payload.DownBytes != 2048 {
		t.Errorf("unexpected payload: %+v", payload)
	}
	if payload.UptimeSeconds != 42 {
		t.Errorf("uptime_seconds=%v, want 42", payload.UptimeSeconds)
	}
}

// TestHandlerLogsJSON 验证 /api/logs 返回 JSON 数组，且预置事件正确出现。
func TestHandlerLogsJSON(t *testing.T) {
	events := stats.NewEventLog(10)
	events.Append(stats.Event{
		Time: "2026-09-04 13:00:00", Kind: "open", Proto: "tcp",
		Client: "1.2.3.4:5555", Target: "example.com:443", ConnID: 1,
	})
	events.Append(stats.Event{
		Time: "2026-09-04 13:00:01", Kind: "close", Proto: "tcp",
		Client: "1.2.3.4:5555", Target: "example.com:443",
		UpBytes: 100, DownBytes: 200, DurationMs: 1234, ConnID: 1,
	})

	s := &Server{Events: events}
	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type=%q, want application/json", ct)
	}
	var got []stats.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON array: %v (%s)", err, rec.Body.String())
	}
	if len(got) != 2 {
		t.Fatalf("len(events)=%d, want 2", len(got))
	}
	if got[0].Kind != "open" || got[0].Target != "example.com:443" {
		t.Errorf("event[0]=%+v", got[0])
	}
	if got[1].Kind != "close" || got[1].DownBytes != 200 || got[1].DurationMs != 1234 {
		t.Errorf("event[1]=%+v", got[1])
	}
}

// TestHandlerLogsNilEventsReturnsEmptyArray 验证 Events 为 nil 时 /api/logs 返回 []。
func TestHandlerLogsNilEventsReturnsEmptyArray(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("body=%q, want []", body)
	}
	var got []stats.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON array: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len=%d, want 0", len(got))
	}
}

// TestHandlerIndexHTML 验证 GET / 返回 200、text/html，且包含服务端注入的已知文案。
func TestHandlerIndexHTML(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type=%q, want text/html", ct)
	}
	body := rec.Body.String()
	for _, key := range []string{i18n.KeyWebTitle, i18n.KeyWebPerfSection, i18n.KeyWebLogsSection, i18n.KeyWebColTime} {
		if label := i18n.T(key); label != "" && !strings.Contains(body, label) {
			t.Errorf("HTML missing injected label %q (key %s)", label, key)
		}
	}
}
