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
	"fmt"
	"net"
	"strings"
)

// targetMatchesListener resolves a configured target and reports whether it
// points back to the active listener. Wildcard listeners cover every local
// interface address, while specifically bound listeners only match that IP.
func targetMatchesListener(ctx context.Context, target string, listener net.Addr) bool {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return false
	}
	listenIP, listenPort, ok := networkAddress(listener)
	if !ok || port != fmt.Sprint(listenPort) {
		return false
	}

	host = strings.Trim(host, "[]")
	if ip := net.ParseIP(host); ip != nil {
		return listenerContainsIP(listenIP, ip)
	}
	resolved, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return false
	}
	for _, addr := range resolved {
		if listenerContainsIP(listenIP, net.IP(addr.AsSlice())) {
			return true
		}
	}
	return false
}

// connMatchesListener performs the post-dial half of the self-target guard.
func connMatchesListener(conn net.Conn, listener net.Addr) bool {
	remoteIP, remotePort, ok := networkAddress(conn.RemoteAddr())
	if !ok {
		return false
	}
	listenIP, listenPort, ok := networkAddress(listener)
	return ok && remotePort == listenPort && listenerContainsIP(listenIP, remoteIP)
}

func networkAddress(addr net.Addr) (net.IP, int, bool) {
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP, a.Port, true
	case *net.UDPAddr:
		return a.IP, a.Port, true
	default:
		return nil, 0, false
	}
}

func listenerContainsIP(listenIP, targetIP net.IP) bool {
	if listenIP.IsUnspecified() {
		return isLocalIP(targetIP)
	}
	return listenIP.Equal(targetIP)
}

func isLocalIP(target net.IP) bool {
	if target.IsLoopback() {
		return true
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(addr.String())
		if err == nil && ip.Equal(target) {
			return true
		}
	}
	return false
}
