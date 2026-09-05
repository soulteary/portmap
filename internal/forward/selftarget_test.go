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
	for _, target := range []string{"127.0.0.1:12345", "127.0.0.1:012345", "localhost:12345"} {
		if !targetMatchesListener(ctx, target, listener, false) {
			t.Fatalf("targetMatchesListener(%q) = false, want true", target)
		}
	}
	for _, target := range []string{"127.0.0.1:12346", "192.0.2.1:12345", "malformed"} {
		if targetMatchesListener(ctx, target, listener, false) {
			t.Fatalf("targetMatchesListener(%q) = true, want false", target)
		}
	}
}

func TestWildcardListenerOnlyMatchesItsAddressFamily(t *testing.T) {
	ctx := context.Background()
	if !targetMatchesListener(ctx, "127.0.0.1:12345", &net.TCPAddr{IP: net.IPv4zero, Port: 12345}, false) {
		t.Fatal("IPv4 wildcard did not match an IPv4 loopback target")
	}
	if targetMatchesListener(ctx, "[::1]:12345", &net.TCPAddr{IP: net.IPv4zero, Port: 12345}, false) {
		t.Fatal("IPv4 wildcard matched an IPv6 loopback target")
	}
	if targetMatchesListener(ctx, "127.0.0.1:12345", &net.TCPAddr{IP: net.IPv6unspecified, Port: 12345}, false) {
		t.Fatal("IPv6 wildcard matched an IPv4 loopback target")
	}
	if !targetMatchesListener(ctx, "127.0.0.1:12345", &net.TCPAddr{IP: net.IPv6unspecified, Port: 12345}, true) {
		t.Fatal("dual-stack IPv6 wildcard did not match an IPv4 loopback target")
	}
	if !targetMatchesListener(ctx, "0.0.0.0:12345", &net.TCPAddr{IP: net.IPv4zero, Port: 12345}, false) {
		t.Fatal("IPv4 wildcard did not match an unspecified IPv4 target")
	}
	if !targetMatchesListener(ctx, "0.0.0.0:12345", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}, false) {
		t.Fatal("IPv4 loopback listener did not match an unspecified target")
	}
}

func TestResolvedTargetAddressMatchesListener(t *testing.T) {
	listener := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	target := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	if !addressMatchesListener(target, listener, false) {
		t.Fatal("resolved target did not match listener")
	}
}

func TestScopedIPv6ZonesRemainDistinct(t *testing.T) {
	ip := net.ParseIP("fe80::1")
	listener := &net.TCPAddr{IP: ip, Port: 12345, Zone: "eth0"}
	if addressMatchesListener(&net.TCPAddr{IP: ip, Port: 12345, Zone: "eth1"}, listener, false) {
		t.Fatal("same scoped IPv6 bytes on different zones matched")
	}
	if !addressMatchesListener(&net.TCPAddr{IP: ip, Port: 12345, Zone: "eth0"}, listener, false) {
		t.Fatal("same scoped IPv6 address and zone did not match")
	}
}

func TestIPv6ZoneNameAndIndexMatch(t *testing.T) {
	interfaces, err := net.Interfaces()
	if err != nil || len(interfaces) == 0 {
		t.Skip("no network interface available")
	}
	iface := interfaces[0]
	ip := net.ParseIP("fe80::1")
	listener := &net.UDPAddr{IP: ip, Port: 12345, Zone: iface.Name}
	target := &net.UDPAddr{IP: ip, Port: 12345, Zone: strconv.Itoa(iface.Index)}
	if !addressMatchesListener(target, listener, false) {
		t.Fatalf("zone aliases %q and %d did not match", iface.Name, iface.Index)
	}
}

func TestNilTargetIPUsesListenerLoopbackFamily(t *testing.T) {
	listener := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 12345}
	target := &net.UDPAddr{IP: nil, Port: 12345}
	if !addressMatchesListener(target, listener, false) {
		t.Fatal("nil target IP did not normalize to IPv4 loopback")
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

func TestTCPForwardRejectsUnspecifiedSelfTarget(t *testing.T) {
	port := freePort(t)
	listenAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	targetAddr := net.JoinHostPort("0.0.0.0", strconv.Itoa(port))
	srv := New(Config{Listen: listenAddr, Target: targetAddr, Network: "tcp"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := srv.ListenAndServe(ctx); err == nil || !strings.Contains(err.Error(), targetAddr) {
		t.Fatalf("ListenAndServe() error = %v, want unspecified self-target error", err)
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
