package main

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func TestForwardSSHValidation(t *testing.T) {
	for _, tc := range []struct {
		name, upstream, mode, proto, host string
		explicit, fail                    bool
	}{
		{name: "default loopback", upstream: "ssh://user@server.example", mode: "go", proto: "tcp", host: "127.0.0.1"},
		{name: "explicit wildcard", upstream: "ssh://user@server.example", mode: "go", proto: "tcp", host: "", explicit: true},
		{name: "explicit IPv6", upstream: "ssh://user@server.example", mode: "go", proto: "tcp", host: "::1", explicit: true},
		{name: "direct unchanged", mode: "go", proto: "tcp"},
		{name: "UDP rejected", upstream: "ssh://user@server.example", mode: "go", proto: "udp", fail: true},
		{name: "socat rejected", upstream: "ssh://user@server.example", mode: "socat", proto: "tcp", fail: true},
		{name: "SOCKS rejected", upstream: "socks5://server.example", mode: "go", proto: "tcp", fail: true},
		{name: "HTTP rejected", upstream: "http://server.example", mode: "go", proto: "tcp", fail: true},
		{name: "invalid URL", upstream: "ssh://user:private-value@server.example:bad", mode: "go", proto: "tcp", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opt := defaultForwardOptions()
			opt.upstream = tc.upstream
			opt.mode = tc.mode
			opt.proto = tc.proto
			opt.listenHostSet = tc.explicit
			if tc.explicit {
				opt.listenHost = tc.host
			}
			err := validateForwardOptions(&opt)
			if (err != nil) != tc.fail {
				t.Fatalf("validation error=%v", err)
			}
			if !tc.fail && opt.listenHost != tc.host {
				t.Fatalf("host=%q want=%q", opt.listenHost, tc.host)
			}
			if err != nil && strings.Contains(err.Error(), "private-value") {
				t.Fatal("credential leaked")
			}
		})
	}
	opt := defaultForwardOptions()
	opt.upstream = "ssh://user@server.example"
	opt.upstreamKeepaliveMaxFailures = -1
	if err := validateForwardOptions(&opt); err == nil {
		t.Fatal("negative failures accepted")
	}
}

func TestForwardSSHConfigLayoutsAndPrecedence(t *testing.T) {
	const fields = `listen_port: 10024
upstream: ssh://user@server.example
upstream_identity: ~/example_key
upstream_identity_passphrase: fixture-passphrase
upstream_known_hosts: ~/example_hosts
upstream_insecure: true
upstream_agent: false
upstream_agent_socket: ~/example_socket
upstream_keepalive: -1s
upstream_keepalive_max_failures: 4
`
	for _, nested := range []bool{false, true} {
		t.Run(map[bool]string{false: "flat", true: "nested"}[nested], func(t *testing.T) {
			text := fields
			if nested {
				text = "forward:\n  " + strings.ReplaceAll(strings.TrimSuffix(fields, "\n"), "\n", "\n  ") + "\n"
			}
			path := writeTempConfig(t, text)
			top, err := loadTopConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			opt := defaultForwardOptions()
			if err := applyForwardConfig(&opt, top.Forward[0], nil); err != nil {
				t.Fatal(err)
			}
			want := upstreamOptions{
				upstream: "ssh://user@server.example", upstreamIdentity: "~/example_key",
				upstreamIdentityPassphrase: "fixture-passphrase", upstreamKnownHosts: "~/example_hosts",
				upstreamInsecure: true, upstreamAgent: false, upstreamAgentSocket: "~/example_socket",
				upstreamKeepalive: -time.Second, upstreamKeepaliveMaxFailures: 4,
			}
			if opt.listenPort != 10024 || opt.upstreamOptions != want {
				t.Fatalf("upstream fields were not merged")
			}
			legacy, err := loadForwardConfig(path)
			if err != nil {
				t.Fatal(err)
			}
			other := defaultForwardOptions()
			if err := mergeConfig(&other, legacy, nil); err != nil {
				t.Fatal(err)
			}
			if opt != other {
				t.Fatal("legacy merge differs from nested merge")
			}
			opt.upstreamAgent = true
			opt.upstreamInsecure = false
			opt.upstreamKeepalive = 0
			opt.upstreamKeepaliveMaxFailures = 0
			opt.upstream = ""
			want = opt.upstreamOptions
			explicit := map[string]bool{"upstream-agent": true, "upstream-insecure": true, "upstream-keepalive": true, "upstream-keepalive-max-failures": true, "upstream": true}
			if err := applyForwardConfig(&opt, top.Forward[0], explicit); err != nil {
				t.Fatal(err)
			}
			if opt.upstreamOptions != want {
				t.Fatal("explicit zero/false/empty CLI values lost")
			}
		})
	}
}

func TestForwardSSHListenHostPresence(t *testing.T) {
	for _, host := range []string{"", "0.0.0.0", "::1"} {
		path := writeTempConfig(t, "forward:\n  upstream: ssh://user@server.example\n  listen_host: '"+host+"'\n")
		top, err := loadTopConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		opt := defaultForwardOptions()
		if err := applyForwardConfig(&opt, top.Forward[0], nil); err != nil {
			t.Fatal(err)
		}
		if err := normalizeForwardOptions(&opt); err != nil {
			t.Fatal(err)
		}
		if opt.listenHost != host {
			t.Fatalf("explicit host %q replaced with %q", host, opt.listenHost)
		}
	}
}

func TestPrepareForwardSSHCredentialsAndRedaction(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE", "env-passphrase")
	t.Setenv("PORTMAP_UPSTREAM_PASSWORD", "env-password")
	opt := defaultForwardOptions()
	opt.upstream = "ssh://user@server.example"
	opt.upstreamIdentity = "~/example_key"
	opt.upstreamIdentityPassphrase = "yaml-passphrase"
	opt.upstreamKnownHosts = "~/example_hosts"
	opt.upstreamAgentSocket = "~/example_socket"
	opt.upstreamAgent = false
	if err := prepareForwardUpstream(&opt); err != nil {
		t.Fatal(err)
	}
	u := opt.preparedUpstream
	if u == nil || u.IdentityPassphrase != "env-passphrase" || !u.DisableAgent || strings.HasPrefix(u.IdentityFile, "~") || strings.HasPrefix(u.KnownHostsFile, "~") || strings.HasPrefix(u.AgentSocket, "~") {
		t.Fatal("credential resolution failed")
	}
	for _, secret := range []string{"env-passphrase", "yaml-passphrase", "example_key", "example_socket"} {
		if strings.Contains(opt.String(), secret) {
			t.Fatal("configuration summary leaked credentials")
		}
	}
	if err := prepareForwardUpstream(&opt); err != nil || opt.preparedUpstream != u {
		t.Fatal("preparation is not idempotent")
	}
}

func TestPrepareForwardSSHPasswordAndInvalidURL(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("PORTMAP_UPSTREAM_PASSWORD", "env-password")
	opt := defaultForwardOptions()
	opt.upstream = "ssh://user@server.example"
	if err := prepareForwardUpstream(&opt); err != nil {
		t.Fatal(err)
	}
	if opt.preparedUpstream.Password != "env-password" {
		t.Fatal("password environment not resolved")
	}
	opt = defaultForwardOptions()
	opt.upstream = "ssh://user:private-value@server.example:bad"
	if err := prepareForwardUpstream(&opt); err == nil || strings.Contains(err.Error(), "private-value") {
		t.Fatal("invalid credential-bearing URL not rejected safely")
	}
}

func TestForwardSSHCLIAndMultiValidation(t *testing.T) {
	for _, argv := range [][]string{
		{"forward", "-upstream", "ssh://user@server.example", "-proto", "udp"},
		{"forward", "-upstream", "ssh://user@server.example", "-mode", "socat"},
		{"forward", "-upstream", "http://server.example"},
		{"forward", "-upstream", "ssh://user@server.example", "-upstream-keepalive-max-failures", "-1"},
	} {
		if err := run(argv); err == nil {
			t.Fatalf("invalid CLI accepted: %v", argv)
		}
	}
	path := writeTempConfig(t, "forward:\n  - listen_port: 10024\n    upstream: ssh://user@server.example\n    proto: udp\n  - listen_port: 10025\n")
	if err := run([]string{"forward", "-config", path}); err == nil {
		t.Fatal("invalid multi-instance upstream accepted")
	}
}

func TestForwardSSHStartupFailures(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("PORTMAP_UPSTREAM_PASSWORD", "")
	for _, kind := range []string{"bind", "stats", "web", "identity"} {
		t.Run(kind, func(t *testing.T) {
			opt := defaultForwardOptions()
			opt.listenHost = "127.0.0.1"
			opt.listenHostSet = true
			opt.listenPort = freeLoopbackPort(t)
			opt.upstream = "ssh://user@server.example"
			opt.upstreamAgent = false
			// A fixture password permits lazy transport construction without network I/O.
			opt.upstream = "ssh://user:fixture-password@server.example"
			switch kind {
			case "bind":
				opt.listenPort = busyListener(t).Addr().(*net.TCPAddr).Port
			case "stats":
				opt.statsAddr = busyListener(t).Addr().String()
			case "web":
				opt.webAddr = busyListener(t).Addr().String()
			case "identity":
				opt.upstreamIdentity = t.TempDir() + "/missing"
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := runGo(ctx, opt); err == nil {
				t.Fatal("startup failure ignored")
			}
		})
	}
}

func TestForwardSSHMultiPreparationFailure(t *testing.T) {
	a := defaultForwardOptions()
	b := defaultForwardOptions()
	b.upstream = "ssh://user:fixture-password@server.example:bad"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := startForwardInstances(ctx, []options{a, b}); err == nil {
		t.Fatal("multi-instance preparation failure ignored")
	}
}

func TestForwardSSHInvalidKeepaliveConfig(t *testing.T) {
	for _, nested := range []bool{true, false} {
		text := "upstream_keepalive: invalid\n"
		if nested {
			text = "forward:\n  upstream_keepalive: invalid\n"
		}
		path := writeTempConfig(t, text)
		cfg, err := loadForwardConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		opt := defaultForwardOptions()
		if err := mergeConfig(&opt, cfg, nil); err == nil {
			t.Fatal("invalid keepalive accepted")
		}
	}
}
