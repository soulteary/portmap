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
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/stats"
)

func TestForwardLoggingAndStats(t *testing.T) {
	var logs bytes.Buffer
	s := New(Config{Logger: log.New(&logs, "", 0), Debug: true})
	s.infof("info")
	s.debugf("debug")
	if logs.Len() == 0 {
		t.Fatal("missing logs")
	}
	before := logs.Len()
	s.cfg.Quiet = true
	s.infof("hidden")
	if logs.Len() != before {
		t.Fatal("quiet mode leaked log")
	}
	if s.Stats() != s.stats {
		t.Fatal("stats mismatch")
	}
	if err := New(Config{Network: "invalid"}).ListenAndServe(context.Background()); err == nil {
		t.Fatal("invalid network accepted")
	}
}
func TestForwardAddressEdgeCases(t *testing.T) {
	ctx := context.Background()
	a := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 80}
	if targetMatchesListener(ctx, "localhost:80", &net.IPAddr{}, false) {
		t.Fatal("non transport listener matched")
	}
	if targetMatchesListener(ctx, "invalid host:80", a, false) {
		t.Fatal("invalid DNS matched")
	}
	if targetMatchesListener(ctx, "localhost:not-a-service", a, false) {
		t.Fatal("invalid port matched")
	}
	if _, err := resolvePort(ctx, "tcp4", "http"); err != nil {
		t.Fatal(err)
	}
	if addressMatchesListener(&net.IPAddr{}, a, false) || addressMatchesListener(a, &net.IPAddr{}, false) {
		t.Fatal("invalid address matched")
	}
	if listenerContainsIP(net.IPv4zero, net.ParseIP("192.0.2.254"), "", "", false) {
		t.Fatal("nonlocal IP matched wildcard")
	}
	if listenerContainsIP(net.IPv6unspecified, net.ParseIP("fe80::1"), "zone-a", "zone-b", false) {
		t.Fatal("different zone matched")
	}
	if listenerContainsIP(net.IPv6loopback, net.IPv6unspecified, "", "", false) != true {
		t.Fatal("unspecified IPv6 should map to loopback")
	}
	if _, zone := parseIPZone("fe80::1%invalid-interface"); zone != "invalid-interface" {
		t.Fatal(zone)
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range addrs {
		ip, _, err := net.ParseCIDR(a.String())
		if err == nil && !isLocalIP(ip) {
			t.Fatalf("local IP missed: %v", ip)
		}
	}
}

type acceptFailure struct {
	err   error
	calls int
}

func (l *acceptFailure) Accept() (net.Conn, error) {
	l.calls++
	if l.calls == 1 {
		return nil, l.err
	}
	return nil, io.ErrClosedPipe
}
func (*acceptFailure) Close() error   { return nil }
func (*acceptFailure) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 54321} }

type acceptTimeout struct{}

func (acceptTimeout) Error() string   { return "accept timeout" }
func (acceptTimeout) Timeout() bool   { return true }
func (acceptTimeout) Temporary() bool { return true }
func TestForwardAcceptFailures(t *testing.T) {
	for _, err := range []error{acceptTimeout{}, io.ErrClosedPipe} {
		ctx, cancel := context.WithCancel(context.Background())
		s := New(Config{Target: "127.0.0.1:1", Quiet: true})
		ln := &acceptFailure{err: err}
		got := s.serveTCPListener(ctx, ctx, cancel, ln)
		cancel()
		if !errors.Is(got, io.ErrClosedPipe) {
			t.Fatal(got)
		}
	}
}
func udpManager(t *testing.T) *udpSessions {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	target, stop := startUDPEcho(t)
	t.Cleanup(stop)
	addr, err := net.ResolveUDPAddr("udp", target)
	if err != nil {
		t.Fatal(err)
	}
	m := &udpSessions{table: make(map[string]*udpSession), server: New(Config{Quiet: true, Events: stats.NewEventLog(100), MaxConns: 100}), conn: conn, target: addr, idle: time.Second}
	t.Cleanup(func() { m.closeAll(); m.server.wg.Wait(); _ = conn.Close() })
	return m
}
func TestUDPConcurrentSessionCreation(t *testing.T) {
	m := udpManager(t)
	client := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.get(context.Background(), client); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got := m.server.TotalConns(); got != 1 {
		t.Fatalf("duplicate sessions: %d", got)
	}
}
func TestUDPForwardFailuresAndRebuild(t *testing.T) {
	for _, kind := range []string{"limit", "dial", "retry-dial", "oversize", "rebuild"} {
		t.Run(kind, func(t *testing.T) {
			m := udpManager(t)
			client := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
			payload := []byte("ping")
			switch kind {
			case "limit":
				m.server.sem = make(chan struct{}, 1)
				m.server.sem <- struct{}{}
			case "dial":
				m.target = &net.UDPAddr{IP: net.IP{1}, Port: 1234}
			case "retry-dial", "rebuild":
				raw, err := net.DialUDP("udp", nil, m.target)
				if err != nil {
					t.Fatal(err)
				}
				_ = raw.Close()
				done := make(chan struct{})
				close(done)
				m.table[client.String()] = &udpSession{dst: raw, client: client, cancel: func() {}, done: done}
				if kind == "retry-dial" {
					m.target = &net.UDPAddr{IP: net.IP{1}, Port: 1234}
				}
			case "oversize":
				payload = make([]byte, 65536)
			}
			m.forward(context.Background(), client, payload)
			assertUDPFailureSnapshot(t, kind, m.server.Snapshot())
		})
	}
}
func TestUDPListenAndResolveFailures(t *testing.T) {
	for _, cfg := range []Config{{Listen: "invalid", Network: "udp"}, {Listen: "127.0.0.1:0", Target: "invalid", Network: "udp"}} {
		ctx, cancel := context.WithCancel(context.Background())
		if err := New(cfg).ListenAndServe(ctx); err == nil {
			t.Fatal("invalid UDP setup accepted")
		}
		cancel()
	}
}

func assertUDPFailureSnapshot(t *testing.T, kind string, snap stats.Snapshot) {
	t.Helper()
	switch kind {
	case "limit":
		if snap.RejectedConns != 1 {
			t.Fatal(snap)
		}
	case "dial", "retry-dial":
		if snap.DialErrors != 1 {
			t.Fatal(snap)
		}
	case "oversize":
		if snap.UpBytes != 0 {
			t.Fatal(snap)
		}
	case "rebuild":
		if snap.UpBytes != 4 {
			t.Fatal(snap)
		}
	}
}
