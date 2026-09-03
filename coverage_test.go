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
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/proxy"
)

// strp/intp/boolp 是构造指针字段的便捷函数（配置结构体字段均为指针）。
func strp(s string) *string { return &s }
func intp(i int) *int       { return &i }
func boolp(b bool) *bool    { return &b }

// TestApplyForwardConfig 覆盖 applyForwardConfig 的全部字段合并、flag 优先、
// nil 安全、以及非法 duration 的两条错误分支。
func TestApplyForwardConfig(t *testing.T) {
	t.Run("nil 配置安全返回", func(t *testing.T) {
		opt := options{listenPort: 22}
		if err := applyForwardConfig(&opt, nil, map[string]bool{}); err != nil {
			t.Fatalf("nil 配置不应报错: %v", err)
		}
		if opt.listenPort != 22 {
			t.Error("nil 配置不应修改字段")
		}
	})

	t.Run("全部字段合并", func(t *testing.T) {
		opt := options{}
		fc := &ForwardConfig{
			ListenPort:  intp(2200),
			ListenHost:  strp("0.0.0.0"),
			Target:      strp("10.1.1.1:22"),
			Mode:        strp("socat"),
			Proto:       strp("udp"),
			ReuseAddr:   boolp(true),
			Sudo:        boolp(true),
			MaxConns:    intp(128),
			LogLevel:    strp("debug"),
			Quiet:       boolp(true),
			DialTimeout: strp("15s"),
			IdleTimeout: strp("90s"),
		}
		if err := applyForwardConfig(&opt, fc, map[string]bool{}); err != nil {
			t.Fatalf("applyForwardConfig: %v", err)
		}
		want := options{
			listenPort:  2200,
			listenHost:  "0.0.0.0",
			target:      "10.1.1.1:22",
			mode:        "socat",
			proto:       "udp",
			reuseAddr:   true,
			useSudo:     true,
			maxConns:    128,
			logLevel:    "debug",
			quiet:       true,
			dialTimeout: 15 * time.Second,
			idleTimeout: 90 * time.Second,
		}
		if opt != want {
			t.Fatalf("合并结果=%+v，期望 %+v", opt, want)
		}
	})

	t.Run("命令行显式设置的字段优先", func(t *testing.T) {
		opt := options{listenPort: 8080, target: "keep:1"}
		fc := &ForwardConfig{ListenPort: intp(9000), Target: strp("override:2")}
		setFlags := map[string]bool{"listen-port": true, "target": true}
		if err := applyForwardConfig(&opt, fc, setFlags); err != nil {
			t.Fatalf("applyForwardConfig: %v", err)
		}
		if opt.listenPort != 8080 || opt.target != "keep:1" {
			t.Fatalf("命令行应优先: %+v", opt)
		}
	})

	t.Run("非法 dial_timeout 返回错误", func(t *testing.T) {
		opt := options{}
		fc := &ForwardConfig{DialTimeout: strp("nope")}
		if err := applyForwardConfig(&opt, fc, map[string]bool{}); err == nil {
			t.Fatal("非法 dial_timeout 应返回错误")
		}
	})

	t.Run("非法 idle_timeout 返回错误", func(t *testing.T) {
		opt := options{}
		fc := &ForwardConfig{IdleTimeout: strp("nope")}
		if err := applyForwardConfig(&opt, fc, map[string]bool{}); err == nil {
			t.Fatal("非法 idle_timeout 应返回错误")
		}
	})
}

// TestNormalizeForwardOptions 覆盖归一化与各校验错误分支，并验证 proto/log_level
// 被规范化为小写。
func TestNormalizeForwardOptions(t *testing.T) {
	base := func() options {
		return options{listenPort: 22, target: "127.0.0.1:2222", proto: "TCP", logLevel: "INFO"}
	}

	t.Run("合法参数归一化大小写", func(t *testing.T) {
		opt := base()
		if err := normalizeForwardOptions(&opt); err != nil {
			t.Fatalf("normalizeForwardOptions: %v", err)
		}
		if opt.proto != "tcp" || opt.logLevel != "info" {
			t.Fatalf("未归一化: proto=%q log=%q", opt.proto, opt.logLevel)
		}
	})

	cases := []struct {
		name   string
		mutate func(*options)
	}{
		{"端口为 0", func(o *options) { o.listenPort = 0 }},
		{"端口超范围", func(o *options) { o.listenPort = 70000 }},
		{"target 为空", func(o *options) { o.target = "  " }},
		{"未知 proto", func(o *options) { o.proto = "sctp" }},
		{"idle 为负", func(o *options) { o.idleTimeout = -time.Second }},
		{"max-conns 为负", func(o *options) { o.maxConns = -1 }},
		{"dial 为负", func(o *options) { o.dialTimeout = -time.Second }},
		{"未知 log-level", func(o *options) { o.logLevel = "trace" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opt := base()
			c.mutate(&opt)
			if err := normalizeForwardOptions(&opt); err == nil {
				t.Fatalf("%s 应返回错误", c.name)
			}
		})
	}
}

// TestBuildProxyUpstream 覆盖直连（空 upstream）、非法 URL、以及成功解析并回填
// SSH 专用字段三条路径。
func TestBuildProxyUpstream(t *testing.T) {
	t.Run("空 upstream 返回直连 nil", func(t *testing.T) {
		u, err := buildProxyUpstream(&proxyOptions{upstream: "  "})
		if err != nil {
			t.Fatalf("空 upstream 不应报错: %v", err)
		}
		if u != nil {
			t.Fatalf("空 upstream 应返回 nil，实际 %+v", u)
		}
	})

	t.Run("非法 upstream URL 返回错误", func(t *testing.T) {
		if _, err := buildProxyUpstream(&proxyOptions{upstream: "ftp://host:21"}); err == nil {
			t.Fatal("非法 scheme 应返回错误")
		}
	})

	t.Run("成功解析并回填 SSH 字段", func(t *testing.T) {
		opt := &proxyOptions{
			upstream:                     "ssh://root@host:22",
			upstreamIdentity:             "/tmp/id_rsa",
			upstreamIdentityPassphrase:   "s3cret",
			upstreamKnownHosts:           "/tmp/known_hosts",
			upstreamInsecure:             true,
			upstreamKeepalive:            45 * time.Second,
			upstreamKeepaliveMaxFailures: 5,
		}
		u, err := buildProxyUpstream(opt)
		if err != nil {
			t.Fatalf("buildProxyUpstream: %v", err)
		}
		if u == nil {
			t.Fatal("期望非 nil 上游配置")
		}
		if u.Scheme != proxy.UpstreamSchemeSSH || u.Addr != "host:22" {
			t.Fatalf("scheme/addr 错误: %+v", u)
		}
		if u.IdentityFile != "/tmp/id_rsa" || u.KnownHostsFile != "/tmp/known_hosts" || !u.Insecure {
			t.Fatalf("SSH 字段回填错误: %+v", u)
		}
		if u.IdentityPassphrase != "s3cret" {
			t.Fatalf("passphrase 未透传: %+v", u)
		}
		if u.KeepaliveInterval != 45*time.Second || u.KeepaliveMaxFailures != 5 {
			t.Fatalf("keepalive 字段回填错误: %+v", u)
		}
	})
}

// TestNewProxyServer 验证 newProxyServer 把 proxyOptions 与上游配置正确映射到
// proxy.Server 字段。
func TestNewProxyServer(t *testing.T) {
	up := &proxy.UpstreamConfig{Scheme: proxy.UpstreamSchemeSOCKS5, Addr: "127.0.0.1:1080"}
	opt := proxyOptions{
		addr:             "0.0.0.0:8118",
		dialTimeout:      12 * time.Second,
		maxConns:         64,
		handshakeTimeout: 3 * time.Second,
		idleTimeout:      2 * time.Minute,
		allowPublic:      true,
	}
	srv := newProxyServer(opt, up)
	if srv.Addr != "0.0.0.0:8118" || srv.DialTimeout != 12*time.Second || srv.MaxConns != 64 {
		t.Fatalf("基础字段映射错误: %+v", srv)
	}
	if srv.HandshakeTimeout != 3*time.Second || srv.IdleTimeout != 2*time.Minute || !srv.AllowPublic {
		t.Fatalf("安全字段映射错误: %+v", srv)
	}
	if srv.Upstream != up {
		t.Fatalf("上游配置未透传: %+v", srv.Upstream)
	}
}

// TestDefaultOptions 验证两个默认值构造函数返回与 flag 默认值一致的基线。
func TestDefaultOptions(t *testing.T) {
	fo := defaultForwardOptions()
	if fo.listenPort != 22 || fo.target != "127.0.0.1:2222" || fo.mode != "go" || fo.proto != "tcp" ||
		!fo.reuseAddr || fo.dialTimeout != 10*time.Second || fo.logLevel != "info" {
		t.Fatalf("defaultForwardOptions 基线错误: %+v", fo)
	}

	po := defaultProxyOptions()
	if po.addr != "127.0.0.1:1080" || po.dialTimeout != 30*time.Second || po.maxConns != 256 ||
		po.handshakeTimeout != 10*time.Second || po.idleTimeout != 5*time.Minute {
		t.Fatalf("defaultProxyOptions 基线错误: %+v", po)
	}
}

// TestForwardListenAddr 验证监听地址拼接（含 IPv6 的方括号形式）。
func TestForwardListenAddr(t *testing.T) {
	cases := []struct {
		host string
		port int
		want string
	}{
		{"", 22, ":22"},
		{"127.0.0.1", 8022, "127.0.0.1:8022"},
		{"::1", 1080, "[::1]:1080"},
	}
	for _, c := range cases {
		if got := forwardListenAddr(options{listenHost: c.host, listenPort: c.port}); got != c.want {
			t.Errorf("forwardListenAddr(%q,%d)=%q，期望 %q", c.host, c.port, got, c.want)
		}
	}
}

// TestValidateProxyOptions 覆盖 validateProxyOptions 的合法与各负值/空地址错误分支。
func TestValidateProxyOptions(t *testing.T) {
	t.Run("合法参数", func(t *testing.T) {
		if err := validateProxyOptions(&proxyOptions{addr: "127.0.0.1:1080"}); err != nil {
			t.Fatalf("合法参数不应报错: %v", err)
		}
	})
	cases := []struct {
		name string
		opt  proxyOptions
	}{
		{"空地址", proxyOptions{addr: "   "}},
		{"负 dial", proxyOptions{addr: "127.0.0.1:1080", dialTimeout: -time.Second}},
		{"负 max-conns", proxyOptions{addr: "127.0.0.1:1080", maxConns: -1}},
		{"负 handshake", proxyOptions{addr: "127.0.0.1:1080", handshakeTimeout: -time.Second}},
		{"负 idle", proxyOptions{addr: "127.0.0.1:1080", idleTimeout: -time.Second}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opt := c.opt
			if err := validateProxyOptions(&opt); err == nil {
				t.Fatalf("%s 应返回错误", c.name)
			}
		})
	}
}

// TestServeProxyGracefulShutdown 用回环端口验证 serveProxy 在 ctx 取消后触发
// Shutdown 并返回 nil（正常退出，而非错误）。
func TestServeProxyGracefulShutdown(t *testing.T) {
	port := freeLoopbackPort(t)
	srv := newProxyServer(proxyOptions{
		addr:             net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		dialTimeout:      time.Second,
		maxConns:         16,
		handshakeTimeout: time.Second,
		idleTimeout:      time.Second,
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveProxy(ctx, srv, "", false, "", false, nil) }()

	// 轮询等待端口开始监听后再触发关闭。
	waitProxyListening(t, port)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveProxy 优雅关闭应返回 nil，实际 %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serveProxy 未在超时内退出")
	}
}

// TestServeProxyBindErrorReturnsError 验证监听地址被占用时 serveProxy 返回错误。
func TestServeProxyBindErrorReturnsError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占位监听失败: %v", err)
	}
	defer func() { _ = ln.Close() }()
	busyAddr := ln.Addr().String()

	srv := newProxyServer(proxyOptions{addr: busyAddr, dialTimeout: time.Second}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := serveProxy(ctx, srv, "", false, "", false, nil); err == nil {
		t.Fatal("绑定已占用端口应返回错误")
	}
}

func waitProxyListening(t *testing.T, port int) {
	t.Helper()
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("代理未在超时内开始监听 %s", addr)
}
