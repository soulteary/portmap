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

package upstream

import (
	"net"
	"testing"
)

func TestParseIPv6UpstreamDefaultPorts(t *testing.T) {
	for _, tc := range []struct{ scheme, port string }{{"ssh", "22"}, {"http", "3128"}, {"socks5", "1080"}} {
		for _, host := range []string{"[::1]", "[::1]:", "[fe80::1%25test]"} {
			t.Run(tc.scheme+"/"+host, func(t *testing.T) {
				cfg, err := ParseUpstreamURL(tc.scheme + "://" + host)
				if err != nil {
					t.Fatal(err)
				}
				hostname := "::1"
				if host == "[fe80::1%25test]" {
					hostname = "fe80::1%test"
				}
				want := net.JoinHostPort(hostname, tc.port)
				if cfg.Addr != want {
					t.Fatalf("address=%q want=%q", cfg.Addr, want)
				}
			})
		}
	}
}

func TestParseIPv6UpstreamExplicitPort(t *testing.T) {
	for _, scheme := range []string{"ssh", "http", "socks5"} {
		cfg, err := ParseUpstreamURL(scheme + "://[::1]:10024")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Addr != "[::1]:10024" {
			t.Fatalf("explicit port changed: %q", cfg.Addr)
		}
	}
}
