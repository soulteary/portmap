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

package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/telemetry"
)

func TestStatsCommandParsing(t *testing.T) {
	for _, args := range [][]string{{"stats", "-h"}, {"stats", "-lang", "en", "-h"}} {
		if err := run(args); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"stats", "-unknown"}, {"stats", "extra"}, {"stats", "-listen", "bad", "-journal", filepath.Join(t.TempDir(), "journal")}} {
		if err := run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if err := validateStatsEndpoint("bad"); err == nil {
		t.Fatal("accepted bad endpoint")
	}
	if err := validateStatsEndpoint(""); err != nil {
		t.Fatal(err)
	}
	if err := validateStatsEndpoint("unix:///tmp/a"); err != nil {
		t.Fatal(err)
	}
	opt := defaultForwardOptions()
	opt.mode = "socat"
	opt.statsEndpoint = "unix:///tmp/a"
	if err := validateForwardOptions(&opt); err == nil {
		t.Fatal("accepted socat reporting")
	}
	var endpoint string
	if err := mergeStatsEndpoint(&endpoint, "unix:///tmp/a"); err != nil {
		t.Fatal(err)
	}
	if err := mergeStatsEndpoint(&endpoint, "unix:///tmp/b"); err == nil {
		t.Fatal("accepted conflict")
	}
	for _, path := range []string{"/stats", "/metrics", "/api/stats", "/api/logs", "/api/history", "/api/sources", "/"} {
		s, err := telemetry.OpenStore(filepath.Join(t.TempDir(), "s"))
		if err != nil {
			t.Fatal(err)
		}
		rec := httptest.NewRecorder()
		collectorHandler(s).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		_ = s.Close()
		if rec.Code != 200 {
			t.Fatalf("%s: %d", path, rec.Code)
		}
	}
}
func TestCollectorLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dir, err := os.MkdirTemp("", "pmstats")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	endpoint := "unix://" + filepath.Join(dir, "s.sock")
	done := make(chan error, 1)
	go func() { done <- serveStats(ctx, endpoint, filepath.Join(dir, "journal"), "127.0.0.1:0") }()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", filepath.Join(dir, "s.sock"))
		if err == nil {
			_ = conn.Close()
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("collector failed to stop")
	}
	if _, err := os.Stat(filepath.Join(dir, "s.sock")); !os.IsNotExist(err) {
		t.Fatal("socket not removed")
	}
}
func TestCollectorHTTPFailureCancelsIngest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := serveStats(ctx, "tcp://127.0.0.1:0", filepath.Join(t.TempDir(), "journal"), "0.0.0.0:0"); err == nil {
		t.Fatal("accepted public HTTP")
	}
}
func TestReporterUnavailableDoesNotBlockForward(t *testing.T) {
	target, stop := startTestEcho(t)
	defer stop()
	opt := defaultForwardOptions()
	opt.listenHost = "127.0.0.1"
	opt.listenPort = freeLoopbackPort(t)
	opt.target = target
	opt.statsEndpoint = "unix:///nonexistent/portmap.sock"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runGo(ctx, opt) }()
	addr := net.JoinHostPort(opt.listenHost, strconv.Itoa(opt.listenPort))
	var conn net.Conn
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		conn, err = net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if conn == nil {
		t.Fatal("forwarder did not start")
	}
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	_, err := conn.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, make([]byte, 5)); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
}

func startTestEcho(t *testing.T) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { defer func() { _ = conn.Close() }(); _, _ = io.Copy(conn, conn) }()
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func TestStatsEndpointConfigAndOverrides(t *testing.T) {
	endpoint := "unix:///tmp/portmap-test.sock"
	forward := defaultForwardOptions()
	if err := applyForwardConfig(&forward, &ForwardConfig{StatsEndpoint: &endpoint}, nil); err != nil {
		t.Fatal(err)
	}
	if forward.statsEndpoint != endpoint {
		t.Fatal("forward endpoint not applied")
	}
	proxy := defaultProxyOptions()
	if err := applyProxyConfig(&proxy, &ProxyConfig{StatsEndpoint: &endpoint}, nil); err != nil {
		t.Fatal(err)
	}
	if proxy.statsEndpoint != endpoint {
		t.Fatal("proxy endpoint not applied")
	}
	forward.statsEndpoint = "tcp://127.0.0.1:9190"
	if err := applyForwardConfig(&forward, &ForwardConfig{StatsEndpoint: &endpoint}, map[string]bool{"stats-endpoint": true}); err != nil {
		t.Fatal(err)
	}
	if forward.statsEndpoint == endpoint {
		t.Fatal("config overrode explicit flag")
	}
	base := defaultForwardOptions()
	base.statsEndpoint = endpoint
	applyForwardGlobalFlags(&forward, base, map[string]bool{"stats-endpoint": true})
	if forward.statsEndpoint != endpoint {
		t.Fatal("global forward flag ignored")
	}
	applyProxyAdminFlags(&proxy, proxyOptions{statsEndpoint: ""}, map[string]bool{"stats-endpoint": true})
	if proxy.statsEndpoint != "" {
		t.Fatal("explicit disable ignored")
	}
	if _, err := collectForwardAdminOptions([]options{{statsEndpoint: endpoint}, {statsEndpoint: "tcp://127.0.0.1:9190"}}); err == nil {
		t.Fatal("forward endpoints conflict")
	}
	if _, err := collectProxyAdminOptions([]proxyOptions{{statsEndpoint: endpoint}, {statsEndpoint: "tcp://127.0.0.1:9190"}}); err == nil {
		t.Fatal("proxy endpoints conflict")
	}
}

func TestStatsEndpointLegacyConfig(t *testing.T) {
	endpoint := "unix:///tmp/portmap-test.sock"
	for _, content := range []string{"stats_endpoint: " + endpoint + "\nlisten_port: 31008\n", "forward:\n  stats_endpoint: " + endpoint + "\n"} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		config, err := loadForwardConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if config.StatsEndpoint == nil || *config.StatsEndpoint != endpoint {
			t.Fatal("endpoint lost while normalizing")
		}
	}
}
func TestPrivateSocketDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := prepareSocketDirectory(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := prepareSocketDirectory(dir); err == nil {
		t.Fatal("accepted public directory")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	if err := prepareSocketDirectory(link); err == nil {
		t.Fatal("accepted symlink")
	}
}

func TestDefaultCollectorAndHTTPQueries(t *testing.T) {
	dir, err := os.MkdirTemp("", "pmdefault")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := serveStats(ctx, defaultStatsSocket(), filepath.Join(dir, "journal"), ""); err != nil {
		t.Fatal(err)
	}
	if err := serveStats(ctx, "tcp://127.0.0.1:0", filepath.Join(dir, "missing", "journal"), ""); err == nil {
		t.Fatal("accepted bad journal path")
	}
	store, err := telemetry.OpenStore(filepath.Join(dir, "http-journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	addr := "127.0.0.1:" + strconv.Itoa(freeLoopbackPort(t))
	wait, errs := startCollectorHTTP(ctx, cancel, addr, store)
	conn := dialEventually(t, addr)
	_ = conn.Close()
	client := http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + addr + "/api/sources")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	_ = resp.Body.Close()
	cancel()
	wait()
	if err := drainStatsErr(errs); err != nil {
		t.Fatal(err)
	}
}
