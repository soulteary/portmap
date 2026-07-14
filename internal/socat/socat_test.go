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

package socat

import (
	"testing"

	"github.com/soulteary/portmap/internal/i18n"
)

func TestArgsTCP(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true}
	args, err := o.Args()
	if err != nil {
		t.Fatalf("Args: %v", err)
	}
	want := []string{"TCP-LISTEN:22,fork,reuseaddr", "TCP:127.0.0.1:2222"}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d]=%q, want %q", i, args[i], want[i])
		}
	}
}

func TestArgsWithListenHost(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true, ListenHost: "127.0.0.1"}
	args, err := o.Args()
	if err != nil {
		t.Fatalf("Args: %v", err)
	}
	want := []string{"TCP-LISTEN:22,bind=127.0.0.1,fork,reuseaddr", "TCP:127.0.0.1:2222"}
	if len(args) != len(want) {
		t.Fatalf("got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d]=%q, want %q", i, args[i], want[i])
		}
	}
}

// TestArgsEmptyListenHost 验证 ListenHost 为空（含仅空白）时输出与不带 host 完全一致。
func TestArgsEmptyListenHost(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true, ListenHost: "  "}
	args, err := o.Args()
	if err != nil {
		t.Fatalf("Args: %v", err)
	}
	want := []string{"TCP-LISTEN:22,fork,reuseaddr", "TCP:127.0.0.1:2222"}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d]=%q, want %q", i, args[i], want[i])
		}
	}
}

func TestArgsUDP(t *testing.T) {
	o := Options{ListenPort: 53, Target: "127.0.0.1:5353", Fork: true, ReuseAddr: true, Proto: "udp"}
	args, err := o.Args()
	if err != nil {
		t.Fatalf("Args: %v", err)
	}
	want := []string{"UDP-LISTEN:53,fork,reuseaddr", "UDP:127.0.0.1:5353"}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg[%d]=%q, want %q", i, args[i], want[i])
		}
	}
}

func TestArgsInvalidProto(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Proto: "sctp"}
	if _, err := o.Args(); err == nil {
		t.Fatal("expected error for invalid proto")
	}
}

func TestArgsInvalidPort(t *testing.T) {
	o := Options{ListenPort: 0, Target: "127.0.0.1:2222"}
	if _, err := o.Args(); err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestArgsEmptyTarget(t *testing.T) {
	o := Options{ListenPort: 22, Target: "  "}
	if _, err := o.Args(); err == nil {
		t.Fatal("expected error for empty target")
	}
}

// TestStringReusesArgs 验证 String 与 Args 输出一致（单一来源）。
func TestStringReusesArgs(t *testing.T) {
	o := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true}
	if got, want := o.String(), "socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222"; got != want {
		t.Fatalf("String()=%q, want %q", got, want)
	}

	sudo := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true, Sudo: true}
	if got, want := sudo.String(), "sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222"; got != want {
		t.Fatalf("String()=%q, want %q", got, want)
	}

	udp := Options{ListenPort: 53, Target: "127.0.0.1:5353", Fork: true, ReuseAddr: true, Proto: "udp"}
	if got, want := udp.String(), "socat UDP-LISTEN:53,fork,reuseaddr UDP:127.0.0.1:5353"; got != want {
		t.Fatalf("String()=%q, want %q", got, want)
	}

	host := Options{ListenPort: 22, Target: "127.0.0.1:2222", Fork: true, ReuseAddr: true, ListenHost: "127.0.0.1"}
	if got, want := host.String(), "socat TCP-LISTEN:22,bind=127.0.0.1,fork,reuseaddr TCP:127.0.0.1:2222"; got != want {
		t.Fatalf("String()=%q, want %q", got, want)
	}
}

func TestStringInvalid(t *testing.T) {
	i18n.SetLang(i18n.English)
	o := Options{ListenPort: -1, Target: "x"}
	if got := o.String(); got != "<invalid socat options>" {
		t.Fatalf("String()=%q, want invalid marker", got)
	}
}
