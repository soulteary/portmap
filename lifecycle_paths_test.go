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
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/stats"
)

func busyListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln
}
func TestRunGoLifecycle(t *testing.T) {
	opt := defaultForwardOptions()
	opt.listenHost = "127.0.0.1"
	opt.listenPort = freeLoopbackPort(t)
	opt.target = busyListener(t).Addr().String()
	opt.quiet = true
	opt.statsAddr = "127.0.0.1:0"
	opt.webAddr = "127.0.0.1:0"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- runGo(ctx, opt) }()
	waitProxyListening(t, opt.listenPort)
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestRunGoStartupFailures(t *testing.T) {
	for _, kind := range []string{"bind", "stats", "web"} {
		t.Run(kind, func(t *testing.T) {
			opt := defaultForwardOptions()
			opt.listenHost = "127.0.0.1"
			opt.listenPort = freeLoopbackPort(t)
			switch kind {
			case "bind":
				opt.listenPort = busyListener(t).Addr().(*net.TCPAddr).Port
			case "stats":
				opt.statsAddr = busyListener(t).Addr().String()
			case "web":
				opt.webAddr = busyListener(t).Addr().String()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := runGo(ctx, opt); err == nil {
				t.Fatal("startup failure lost")
			}
		})
	}
}
func TestRunSocatSuccessAndFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	opt := defaultForwardOptions()
	opt.listenPort = 8022
	if err := runSocat(context.Background(), opt, map[string]bool{"quiet": true}); err == nil {
		t.Fatal("missing socat accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "socat"), []byte("#!/bin/sh\nexit 0\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := runSocat(context.Background(), opt, nil); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-mode", "socat", "-quiet"}); err != nil {
		t.Fatal(err)
	}
}
func TestCommandEntryRoutes(t *testing.T) {
	old := i18n.Current()
	defer i18n.SetLang(old)
	for _, argv := range [][]string{{"version"}, {"forward", "-h"}, {"proxy", "-h"}, {"-lang", "en", "-version"}, {"proxy", "-version"}} {
		if err := run(argv); err != nil {
			t.Fatalf("%v: %v", argv, err)
		}
	}
	ln := busyListener(t)
	if err := run([]string{"-listen-host", "127.0.0.1", "-listen-port", strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)}); err == nil {
		t.Fatal("occupied forward address accepted")
	}
	if err := run([]string{"proxy", "-addr", ln.Addr().String(), "-web-addr", "127.0.0.1:0"}); err == nil {
		t.Fatal("occupied proxy address accepted")
	}
	if err := run([]string{"proxy", "-upstream", "unknown://host"}); err == nil {
		t.Fatal("invalid upstream accepted")
	}
}
func TestProxyMultiStartupAndValidation(t *testing.T) {
	old := i18n.Current()
	defer i18n.SetLang(old)
	base := defaultProxyOptions()
	base.statsAddr = "127.0.0.1:0"
	base.webAddr = "127.0.0.1:0"
	base.webLogMax = 10
	flags := map[string]bool{"addr": true, "stats-addr": true, "stats-allow-public": true, "web-addr": true, "web-allow-public": true, "web-log-max": true}
	cfg := &Config{Lang: strp("en"), Proxy: ProxyConfigList{{Addr: strp(busyListener(t).Addr().String())}}}
	if err := runProxyMulti(base, cfg, flags); err == nil {
		t.Fatal("busy proxy instance accepted")
	}
	cases := []*ProxyConfig{
		{DialTimeout: strp("bad")}, {Addr: strp("")}, {Upstream: strp("unknown://host")},
	}
	for _, pc := range cases {
		if _, err := buildProxyInstances(base, &Config{Proxy: ProxyConfigList{pc}}, nil); err == nil {
			t.Fatal("invalid instance accepted")
		}
	}
	cfg.Proxy = ProxyConfigList{{Addr: strp("127.0.0.1:12345")}, {Addr: strp("127.0.0.1:12345")}}
	if _, err := buildProxyInstances(base, cfg, nil); err == nil {
		t.Fatal("duplicate listener accepted")
	}
}
func TestMultiAdminStartupFailures(t *testing.T) {
	for _, endpoint := range []string{"stats", "web"} {
		t.Run(endpoint, func(t *testing.T) {
			base := defaultProxyOptions()
			flags := map[string]bool{}
			switch endpoint {
			case "stats":
				base.statsAddr = busyListener(t).Addr().String()
				flags["stats-addr"] = true
			case "web":
				base.webAddr = busyListener(t).Addr().String()
				flags["web-addr"] = true
			}
			cfg := &Config{Proxy: ProxyConfigList{{Addr: strp("127.0.0.1:0")}}}
			if err := runProxyMulti(base, cfg, flags); err == nil {
				t.Fatal("admin startup failure lost")
			}
		})
	}
}
func TestEndpointErrorChannels(t *testing.T) {
	if err := drainStatsErr(makeClosedErrorChannel(nil)); err != nil {
		t.Fatal(err)
	}
	want := errors.New("endpoint failed")
	if err := drainStatsErr(makeClosedErrorChannel(want)); !errors.Is(err, want) {
		t.Fatal(err)
	}
	for _, web := range []bool{false, true} {
		t.Run(map[bool]string{true: "web", false: "stats"}[web], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var wait func()
			var errs <-chan error
			if web {
				wait, errs = startWebEndpoint(ctx, cancel, busyListener(t).Addr().String(), false, stats.NewEventLog(10))
			} else {
				wait, errs = startStatsEndpoint(ctx, cancel, busyListener(t).Addr().String(), false)
			}
			wait()
			if err := drainStatsErr(errs); err == nil {
				t.Fatal("endpoint error discarded")
			}
		})
	}
}
func makeClosedErrorChannel(err error) <-chan error {
	ch := make(chan error, 1)
	if err != nil {
		ch <- err
	}
	close(ch)
	return ch
}
func TestConfigSnapshotBoundaries(t *testing.T) {
	for _, section := range []string{"forward", "proxy"} {
		t.Run(section, func(t *testing.T) {
			if _, err := loadCommandConfig("/does-not-exist/config.yaml", false, section); err == nil {
				t.Fatal("missing file accepted")
			}
			if hasCommandInstances(&Config{}, section) {
				t.Fatal("missing section accepted")
			}
			if hasCommandInstances(&Config{Forward: ForwardConfigList{nil}, Proxy: ProxyConfigList{nil}}, section) {
				t.Fatal("nil instance accepted")
			}
		})
	}
	cfg := &fileConfig{StatsAddr: strp("a"), WebAddr: strp("b"), WebLogMax: intp(12), Lang: strp("fr")}
	opt := options{}
	if err := mergeConfig(&opt, cfg, nil); err != nil {
		t.Fatal(err)
	}
	if opt.statsAddr != "a" || opt.webAddr != "b" || opt.webLogMax != 12 || opt.lang != "fr" {
		t.Fatal(opt)
	}

}

func TestForwardMultiRuntimeStartup(t *testing.T) {
	old := i18n.Current()
	defer i18n.SetLang(old)
	ln := busyListener(t)
	port := ln.Addr().(*net.TCPAddr).Port
	cfg := &Config{Lang: strp("en"), Forward: ForwardConfigList{{ListenHost: strp("127.0.0.1"), ListenPort: intp(port), WebAddr: strp("127.0.0.1:0")}}}
	if err := runForwardMulti(defaultForwardOptions(), cfg, map[string]bool{"listen-port": true}); err == nil {
		t.Fatal("busy multi forward accepted")
	}
	cfg.Forward = ForwardConfigList{{DialTimeout: strp("invalid")}}
	if err := runForwardMulti(defaultForwardOptions(), cfg, nil); err == nil {
		t.Fatal("invalid duration accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	a := defaultForwardOptions()
	b := a
	a.webAddr = "127.0.0.1:9000"
	b.webAddr = "127.0.0.1:9001"
	if err := startForwardInstances(ctx, []options{a, b}); err == nil {
		t.Fatal("inconsistent admin addresses accepted")
	}
}
func TestEntrypointParseErrors(t *testing.T) {
	for _, argv := range [][]string{{"-unknown"}, {"proxy", "-unknown"}} {
		if err := run(argv); err == nil {
			t.Fatal("unknown flag accepted")
		}
	}
	old := os.Args
	os.Args = []string{"portmap", "version"}
	defer func() { os.Args = old }()
	main()
}

func TestPasswordPromptNeverEchoesSecrets(t *testing.T) {
	for _, readErr := range []error{nil, io.ErrClosedPipe} {
		var output bytes.Buffer
		value, err := readPasswordPrompt("password: ", 42, &output, func(fd int) ([]byte, error) {
			if fd != 42 {
				t.Fatal(fd)
			}
			return []byte("private-secret"), readErr
		})
		if !errors.Is(err, readErr) {
			t.Fatal(err)
		}
		if output.String() != "password: \n" {
			t.Fatalf("unexpected prompt output %q", output.String())
		}
		want := "private-secret"
		if readErr != nil {
			want = ""
		}
		if value != want {
			t.Fatal("unexpected password result")
		}
	}
}
