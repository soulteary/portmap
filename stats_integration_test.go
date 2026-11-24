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
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/telemetry"
)

func TestStatsCLIHelper(t *testing.T) {
	raw := os.Getenv("PORTMAP_TEST_CLI")
	if raw == "" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatal(err)
	}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
}
func startStatsCLI(t *testing.T, args []string) {
	t.Helper()
	raw, _ := json.Marshal(args)
	cmd := exec.Command(os.Args[0], "-test.run=^TestStatsCLIHelper$")
	cmd.Env = append(os.Environ(), "PORTMAP_TEST_CLI="+string(raw))
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("CLI failed: %v\n%s", err, output.String())
			}
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			err := <-done
			t.Errorf("CLI shutdown timed out: %v", err)
		}
	})
}
func dialEventually(t *testing.T, addr string) net.Conn {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			return c
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("listener did not start")
	return nil
}
func exchangeStatsData(t *testing.T, c net.Conn) {
	t.Helper()
	_ = c.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(c, make([]byte, 5)); err != nil {
		t.Fatal(err)
	}
}
func waitStatsBytes(t *testing.T, s *telemetry.Store, want int64) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		v := s.Snapshot()
		if v.UpBytes >= want && v.DownBytes >= want && v.ActiveConns > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("missing live statistics: %+v", s.Snapshot())
}
func TestCommandsReportLiveTrafficAcrossCollectorRestart(t *testing.T) {
	for _, mode := range []string{"default", "forward", "proxy"} {
		t.Run(mode, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "pmcli")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			target, stop := startTestEcho(t)
			t.Cleanup(stop)
			port := strconv.Itoa(freeLoopbackPort(t))
			addr := "127.0.0.1:" + port
			endpoint := "unix://" + filepath.Join(dir, "s.sock")
			config := filepath.Join(dir, "config.yaml")
			var args []string
			switch mode {
			case "default":
				content := "forward:\n  listen_host: 127.0.0.1\n  listen_port: " + port + "\n  target: " + target + "\n  stats_endpoint: " + endpoint + "\n"
				if err := os.WriteFile(config, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				args = []string{"-config", config}
			case "forward":
				second := strconv.Itoa(freeLoopbackPort(t))
				content := "forward:\n  - listen_host: 127.0.0.1\n    listen_port: " + port + "\n    target: " + target + "\n  - listen_host: 127.0.0.1\n    listen_port: " + second + "\n    target: " + target + "\n"
				if err := os.WriteFile(config, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				args = []string{"forward", "-config", config, "-stats-endpoint", endpoint}
			case "proxy":
				args = []string{"proxy", "-addr", addr, "-stats-endpoint", endpoint}
			}
			startStatsCLI(t, args)
			conn := dialEventually(t, addr)
			t.Cleanup(func() { _ = conn.Close() })
			if mode == "proxy" {
				statsSOCKSHandshake(t, conn, target)
			}
			exchangeStatsData(t, conn) // no collector exists yet
			journal := filepath.Join(dir, "journal")
			start := func() (*telemetry.Store, func()) {
				store, err := telemetry.OpenStore(journal)
				if err != nil {
					t.Fatal(err)
				}
				ln, err := telemetry.Listen(endpoint)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() { done <- telemetry.Serve(ctx, ln, store) }()
				return store, func() {
					cancel()
					if err := <-done; err != nil {
						t.Error(err)
					}
					_ = store.Close()
				}
			}
			store, stopCollector := start()
			waitStatsBytes(t, store, 5)
			stopCollector()
			exchangeStatsData(t, conn) // keep the same business connection through the outage
			store, stopCollector = start()
			defer stopCollector()
			waitStatsBytes(t, store, 10)
			if got := store.Snapshot(); got.UpBytes != 10 || got.DownBytes != 10 {
				t.Fatalf("duplicate totals: %+v", got)
			}
		})
	}
}

func statsSOCKSHandshake(t *testing.T, conn net.Conn, target string) {
	t.Helper()

	// A SOCKS5 handshake establishes an ordinary bidirectional stream.
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, make([]byte, 2)); err != nil {
		t.Fatal(err)
	}
	host, port, _ := net.SplitHostPort(target)
	p, _ := strconv.Atoi(port)
	request := append([]byte{5, 1, 0, 1}, net.ParseIP(host).To4()...)
	request = append(request, byte(p>>8), byte(p))
	if _, err := conn.Write(request); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 10)
	if _, err := io.ReadFull(conn, reply); err != nil || reply[1] != 0 {
		t.Fatalf("handshake %v %v", reply, err)
	}
}
