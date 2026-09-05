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

package stats

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCountersConcurrent(t *testing.T) {
	c := New()
	const workers = 50
	const iters = 1000

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				c.ConnOpened()
				c.AddUp(2)
				c.AddDown(3)
				c.Reject()
				c.DialError()
				c.ConnClosed()
			}
		}()
	}
	wg.Wait()

	snap := c.Snapshot()
	total := int64(workers * iters)
	if snap.ActiveConns != 0 {
		t.Errorf("ActiveConns=%d, want 0", snap.ActiveConns)
	}
	if snap.TotalConns != total {
		t.Errorf("TotalConns=%d, want %d", snap.TotalConns, total)
	}
	if snap.RejectedConns != total {
		t.Errorf("RejectedConns=%d, want %d", snap.RejectedConns, total)
	}
	if snap.DialErrors != total {
		t.Errorf("DialErrors=%d, want %d", snap.DialErrors, total)
	}
	if snap.UpBytes != total*2 {
		t.Errorf("UpBytes=%d, want %d", snap.UpBytes, total*2)
	}
	if snap.DownBytes != total*3 {
		t.Errorf("DownBytes=%d, want %d", snap.DownBytes, total*3)
	}
	if snap.Uptime <= 0 {
		t.Errorf("Uptime=%s, want > 0", snap.Uptime)
	}
}

func TestSnapshotActiveAccounting(t *testing.T) {
	c := New()
	c.ConnOpened()
	c.ConnOpened()
	c.ConnClosed()
	snap := c.Snapshot()
	if snap.ActiveConns != 1 {
		t.Errorf("ActiveConns=%d, want 1", snap.ActiveConns)
	}
	if snap.TotalConns != 2 {
		t.Errorf("TotalConns=%d, want 2", snap.TotalConns)
	}
}

func TestConnOpenedReturnsProcessUniqueID(t *testing.T) {
	a, b := New(), New()
	first := a.ConnOpened()
	second := b.ConnOpened()
	if first <= 0 || second <= 0 || first == second {
		t.Fatalf("连接编号必须跨实例唯一且非零: first=%d second=%d", first, second)
	}
}

func TestSnapshotAddAggregation(t *testing.T) {
	a := Snapshot{ActiveConns: 1, TotalConns: 2, RejectedConns: 3, DialErrors: 4, UpBytes: 5, DownBytes: 6, Uptime: 10 * time.Second}
	b := Snapshot{ActiveConns: 10, TotalConns: 20, RejectedConns: 30, DialErrors: 40, UpBytes: 50, DownBytes: 60, Uptime: 30 * time.Second}
	got := a.Add(b)
	want := Snapshot{ActiveConns: 11, TotalConns: 22, RejectedConns: 33, DialErrors: 44, UpBytes: 55, DownBytes: 66, Uptime: 30 * time.Second}
	if got != want {
		t.Errorf("Add=%+v, want %+v", got, want)
	}
}

type fakeProvider struct{ snap Snapshot }

func (f fakeProvider) Snapshot() Snapshot { return f.snap }

func TestAggregateMultipleProviders(t *testing.T) {
	providers := []Provider{
		fakeProvider{Snapshot{TotalConns: 1, UpBytes: 100, Uptime: 5 * time.Second}},
		fakeProvider{Snapshot{TotalConns: 2, UpBytes: 200, Uptime: 9 * time.Second}},
		nil, // nil providers must be skipped.
	}
	got := Aggregate(providers)
	if got.TotalConns != 3 || got.UpBytes != 300 || got.Uptime != 9*time.Second {
		t.Errorf("Aggregate=%+v", got)
	}
}

func TestHandlerStatsJSON(t *testing.T) {
	s := &Server{Providers: []Provider{fakeProvider{Snapshot{
		ActiveConns: 2, TotalConns: 7, RejectedConns: 1, DialErrors: 3,
		UpBytes: 1024, DownBytes: 2048, Uptime: 42 * time.Second,
	}}}}

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

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
	if payload.TotalConns != 7 || payload.UpBytes != 1024 || payload.DownBytes != 2048 {
		t.Errorf("unexpected payload: %+v", payload)
	}
	if payload.UptimeSeconds != 42 {
		t.Errorf("uptime_seconds=%v, want 42", payload.UptimeSeconds)
	}
}

func TestHandlerMetricsPrometheus(t *testing.T) {
	s := &Server{Providers: []Provider{fakeProvider{Snapshot{
		TotalConns: 7, UpBytes: 1024, DownBytes: 2048, Uptime: 42 * time.Second,
	}}}}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, want := range []string{
		"# TYPE portmap_total_connections counter",
		"portmap_total_connections 7",
		"portmap_up_bytes 1024",
		"portmap_down_bytes 2048",
		"portmap_uptime_seconds 42",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics output missing %q\n%s", want, body)
		}
	}
}

func TestServerListenAndServeLoopback(t *testing.T) {
	c := New()
	c.ConnOpened()
	c.AddUp(500)
	s := &Server{Addr: "127.0.0.1:0", Providers: []Provider{c}}

	// 手动监听以拿到实际端口（Addr 用 :0）。
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	realAddr := ln.Addr().String()
	_ = ln.Close()

	s.Addr = realAddr
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.ListenAndServe(ctx) }()

	var resp *http.Response
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err = http.Get("http://" + realAddr + "/stats")
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("GET /stats: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "\"up_bytes\": 500") {
		t.Errorf("stats body missing up_bytes: %s", body)
	}

	cancel()
	if err := <-done; err != nil {
		t.Errorf("ListenAndServe returned %v, want nil after ctx cancel", err)
	}
}

func TestServerRefusesNonLoopback(t *testing.T) {
	s := &Server{Addr: "0.0.0.0:0", Providers: []Provider{New()}}
	err := s.ListenAndServe(context.Background())
	if err == nil {
		t.Fatal("expected error for non-loopback address without AllowPublic")
	}
}

func TestServerEmptyAddrDisabled(t *testing.T) {
	s := &Server{Addr: "", Providers: []Provider{New()}}
	if err := s.ListenAndServe(context.Background()); err != nil {
		t.Errorf("empty addr should be a no-op, got %v", err)
	}
}
