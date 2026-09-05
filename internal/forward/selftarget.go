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
)

// targetMatchesListener resolves a configured target and reports whether it
// points back to the active listener. Wildcard listeners cover every local
// interface address, while specifically bound listeners only match that IP.
func targetMatchesListener(ctx context.Context, target string, listener net.Addr, dualStack bool) bool {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return false
	}
	listenIP, listenPort, listenZone, ok := networkAddress(listener)
	if !ok {
		return false
	}
	targetPort, err := resolvePort(ctx, listener.Network(), port)
	if err != nil || targetPort != listenPort {
		return false
	}

	host = strings.Trim(host, "[]")
	if ip, zone := parseIPZone(host); ip != nil {
		return listenerContainsIP(listenIP, ip, listenZone, zone, dualStack)
	}
	resolved, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return false
	}
	for _, addr := range resolved {
		if listenerContainsIP(listenIP, net.IP(addr.AsSlice()), listenZone, "", dualStack) {
			return true
		}
	}
	return false
}

func parseIPZone(host string) (net.IP, string) {
	if percent := strings.LastIndexByte(host, '%'); percent >= 0 {
		return net.ParseIP(host[:percent]), host[percent+1:]
	}
	return net.ParseIP(host), ""
}

func resolvePort(ctx context.Context, network, port string) (int, error) {
	if value, err := strconv.ParseUint(port, 10, 16); err == nil {
		return int(value), nil
	}
	network = strings.TrimSuffix(strings.TrimSuffix(network, "4"), "6")
	return net.DefaultResolver.LookupPort(ctx, network, port)
}

// connMatchesListener performs the post-dial half of the self-target guard.
func connMatchesListener(conn net.Conn, listener net.Addr, dualStack bool) bool {
	return addressMatchesListener(conn.RemoteAddr(), listener, dualStack)
}

func addressMatchesListener(target, listener net.Addr, dualStack bool) bool {
	remoteIP, remotePort, remoteZone, ok := networkAddress(target)
	if !ok {
		return false
	}
	listenIP, listenPort, listenZone, ok := networkAddress(listener)
	return ok && remotePort == listenPort && listenerContainsIP(listenIP, remoteIP, listenZone, remoteZone, dualStack)
}

func networkAddress(addr net.Addr) (net.IP, int, string, bool) {
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP, a.Port, a.Zone, true
	case *net.UDPAddr:
		return a.IP, a.Port, a.Zone, true
	default:
		return nil, 0, "", false
	}
}

func listenerContainsIP(listenIP, targetIP net.IP, listenZone, targetZone string, dualStack bool) bool {
	if targetIP.IsUnspecified() {
		if targetIP.To4() != nil {
			targetIP = net.IPv4(127, 0, 0, 1)
		} else {
			targetIP = net.IPv6loopback
		}
	}
	if listenIP.IsUnspecified() {
		if listenZone != "" && listenZone != targetZone {
			return false
		}
		familyMatches := sameIPFamily(listenIP, targetIP) || (dualStack && listenIP.To4() == nil && targetIP.To4() != nil)
		return familyMatches && isLocalIP(targetIP)
	}
	return listenIP.Equal(targetIP) && listenZone == targetZone
}

func sameIPFamily(a, b net.IP) bool {
	return (a.To4() != nil) == (b.To4() != nil)
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
