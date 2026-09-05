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
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestTargetMatchesListener(t *testing.T) {
	listener := &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	ctx := context.Background()
	for _, target := range []string{"127.0.0.1:12345", "localhost:12345"} {
		if !targetMatchesListener(ctx, target, listener) {
			t.Fatalf("targetMatchesListener(%q) = false, want true", target)
		}
	}
	for _, target := range []string{"127.0.0.1:12346", "192.0.2.1:12345", "malformed"} {
		if targetMatchesListener(ctx, target, listener) {
			t.Fatalf("targetMatchesListener(%q) = true, want false", target)
		}
	}
}

func TestTCPForwardRejectsSelfTarget(t *testing.T) {
	port := freePort(t)
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	srv := New(Config{Listen: addr, Target: addr, Network: "tcp"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.ListenAndServe(ctx); err == nil || !strings.Contains(err.Error(), addr) {
		t.Fatalf("ListenAndServe() error = %v, want self-target error", err)
	}
}

func TestUDPForwardRejectsSelfTarget(t *testing.T) {
	probe, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.LocalAddr().String()
	_ = probe.Close()

	srv := New(Config{Listen: addr, Target: addr, Network: "udp"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.ListenAndServe(ctx); err == nil || !strings.Contains(err.Error(), addr) {
		t.Fatalf("ListenAndServe() error = %v, want self-target error", err)
	}
}
