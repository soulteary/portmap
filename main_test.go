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
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestRunValidation 覆盖 run() 中不会真正启动服务的路径：
// 校验阶段提前返回错误，以及 -version 提前返回 nil。
// go/socat 分支会监听端口或执行外部命令，不在单元测试内触发。
func TestRunValidation(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantErr bool
	}{
		{name: "version 提前返回 nil", argv: []string{"-version"}, wantErr: false},

		{name: "无参数展示 help 返回 nil", argv: []string{}, wantErr: false},

		{name: "端口为 0 非法", argv: []string{"-listen-port", "0"}, wantErr: true},
		{name: "端口超范围非法", argv: []string{"-listen-port", "70000"}, wantErr: true},
		{name: "端口为负非法", argv: []string{"-listen-port", "-1"}, wantErr: true},

		{name: "target 为空非法", argv: []string{"-target", ""}, wantErr: true},

		{name: "未知 proto 非法", argv: []string{"-proto", "sctp"}, wantErr: true},

		{name: "idle-timeout 为负非法", argv: []string{"-idle-timeout", "-1s"}, wantErr: true},
		{name: "max-conns 为负非法", argv: []string{"-max-conns", "-1"}, wantErr: true},
		{name: "dial-timeout 为负非法", argv: []string{"-dial-timeout", "-1s"}, wantErr: true},
		{name: "proxy max-conns 为负非法", argv: []string{"proxy", "-max-conns", "-1"}, wantErr: true},
		{name: "proxy handshake-timeout 为负非法", argv: []string{"proxy", "-handshake-timeout", "-1s"}, wantErr: true},
		{name: "proxy idle-timeout 为负非法", argv: []string{"proxy", "-idle-timeout", "-1s"}, wantErr: true},

		{name: "未知 log-level 非法", argv: []string{"-log-level", "trace"}, wantErr: true},

		// mode 校验位于校验区之后，需保证其它参数合法才能走到 mode 分支。
		{name: "未知 mode 非法", argv: []string{"-mode", "foo"}, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.argv)
			if (err != nil) != tc.wantErr {
				t.Fatalf("run(%v) 返回 err=%v，期望 wantErr=%v", tc.argv, err, tc.wantErr)
			}
		})
	}
}

// TestRunConfig 覆盖 -config 的加载与合并逻辑：
// 借助不合法的合并结果或非法 mode，在真正启动服务前提前返回，
// 从而在不监听端口的情况下断言合并 + flag 优先行为。
func TestRunConfig(t *testing.T) {
	writeCfg := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("写入临时配置失败: %v", err)
		}
		return path
	}

	t.Run("config 文件不存在返回错误", func(t *testing.T) {
		if err := run([]string{"-config", filepath.Join(t.TempDir(), "nope.yaml")}); err == nil {
			t.Fatal("config 文件不存在应返回错误")
		}
	})

	t.Run("config 非法 duration 返回错误", func(t *testing.T) {
		path := writeCfg(t, "dial_timeout: not-a-duration\n")
		if err := run([]string{"-config", path}); err == nil {
			t.Fatal("非法 dial_timeout 应返回错误")
		}
	})

	t.Run("config 提供的非法端口触发校验错误（说明配置已合并）", func(t *testing.T) {
		path := writeCfg(t, "listen_port: 0\n")
		if err := run([]string{"-config", path}); err == nil {
			t.Fatal("配置中 listen_port=0 应被合并并触发校验错误")
		}
	})

	t.Run("命令行 flag 优先于 config（覆盖非法端口）", func(t *testing.T) {
		// 配置文件给出非法端口 0，但命令行显式指定合法端口且用未知 mode 提前返回。
		// 期望：flag 优先使端口校验通过，最终因未知 mode 返回错误（而非端口错误）。
		path := writeCfg(t, "listen_port: 0\n")
		err := run([]string{"-config", path, "-listen-port", "8022", "-mode", "foo"})
		if err == nil {
			t.Fatal("未知 mode 应返回错误")
		}
		if got := err.Error(); !strings.Contains(got, "mode") {
			t.Fatalf("期望因未知 mode 返回错误，实际: %v", got)
		}
	})

	t.Run("config 非法 proto 触发校验错误", func(t *testing.T) {
		path := writeCfg(t, "proto: sctp\n")
		if err := run([]string{"-config", path}); err == nil {
			t.Fatal("配置中非法 proto 应触发校验错误")
		}
	})
}

// TestOptionsString 验证 options.String 输出包含关键字段。
func TestOptionsString(t *testing.T) {
	o := options{
		listenPort:  8022,
		listenHost:  "127.0.0.1",
		target:      "10.0.0.1:22",
		mode:        "go",
		proto:       "tcp",
		reuseAddr:   true,
		dialTimeout: 10 * time.Second,
		maxConns:    128,
		idleTimeout: time.Minute,
		logLevel:    "debug",
		quiet:       false,
		useSudo:     true,
	}
	s := o.String()
	for _, sub := range []string{"mode=go", "proto=tcp", "listen=127.0.0.1:8022", "target=10.0.0.1:22", "reuseaddr=true", "max-conns=128", "log-level=debug", "sudo=true"} {
		if !strings.Contains(s, sub) {
			t.Errorf("String() 缺少 %q: %s", sub, s)
		}
	}
}

// TestSplitSubcommand 覆盖子命令拆分的各分支。
func TestSplitSubcommand(t *testing.T) {
	cases := []struct {
		name     string
		argv     []string
		wantSub  string
		wantRest []string
	}{
		{"空参数默认 forward", nil, "forward", nil},
		{"以 flag 开头默认 forward", []string{"-listen-port", "22"}, "forward", []string{"-listen-port", "22"}},
		{"显式 proxy 子命令", []string{"proxy", "-addr", "x"}, "proxy", []string{"-addr", "x"}},
		{"显式 version 子命令", []string{"version"}, "version", []string{}},
		{"未知位置参数默认 forward", []string{"bogus", "arg"}, "forward", []string{"bogus", "arg"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sub, rest := splitSubcommand(c.argv)
			if sub != c.wantSub {
				t.Fatalf("sub=%q，期望 %q", sub, c.wantSub)
			}
			if len(rest) != len(c.wantRest) {
				t.Fatalf("rest=%v，期望 %v", rest, c.wantRest)
			}
			for i := range c.wantRest {
				if rest[i] != c.wantRest[i] {
					t.Fatalf("rest[%d]=%q，期望 %q", i, rest[i], c.wantRest[i])
				}
			}
		})
	}
}

// TestPreScanLang 覆盖 -lang / --lang / -lang=xx / 子命令跳过 / 未找到 等分支。
func TestPreScanLang(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"空格分隔", []string{"-lang", "zh"}, "zh"},
		{"等号形式", []string{"-lang=ja"}, "ja"},
		{"双横线", []string{"--lang", "fr"}, "fr"},
		{"跳过子命令名", []string{"proxy", "-lang", "de"}, "de"},
		{"未提供值", []string{"-lang"}, ""},
		{"无 lang flag", []string{"-listen-port", "22"}, ""},
		{"-- 之后停止", []string{"--", "-lang", "zh"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := preScanLang(c.argv); got != c.want {
				t.Fatalf("preScanLang(%v)=%q，期望 %q", c.argv, got, c.want)
			}
		})
	}
}

// TestRunVersionSubcommand 验证 version 子命令返回 nil。
func TestRunVersionSubcommand(t *testing.T) {
	if err := run([]string{"version"}); err != nil {
		t.Fatalf("version 子命令应返回 nil，实际 %v", err)
	}
}

// TestRunProxyValidation 覆盖 runProxy 的校验分支：非法上游 URL、负超时、
// -version 提前返回、空地址等，均在监听端口前返回。
func TestRunProxyValidation(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantErr bool
	}{
		{"proxy version 提前返回", []string{"proxy", "-version"}, false},
		{"proxy 空地址非法", []string{"proxy", "-addr", ""}, true},
		{"proxy 负 dial-timeout", []string{"proxy", "-dial-timeout", "-1s"}, true},
		{"proxy 负 handshake-timeout", []string{"proxy", "-handshake-timeout", "-1s"}, true},
		{"proxy 负 idle-timeout", []string{"proxy", "-idle-timeout", "-1s"}, true},
		{"proxy 非法上游 scheme", []string{"proxy", "-upstream", "ftp://host:21"}, true},
		{"proxy 上游缺 host", []string{"proxy", "-upstream", "socks5://"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := run(tc.argv); (err != nil) != tc.wantErr {
				t.Fatalf("run(%v) err=%v，期望 wantErr=%v", tc.argv, err, tc.wantErr)
			}
		})
	}
}

// TestRunProxyConfigLoad 验证 proxy 子命令的 -config 加载与合并。
func TestRunProxyConfigLoad(t *testing.T) {
	writeCfg := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("写入临时配置失败: %v", err)
		}
		return path
	}

	t.Run("config 文件不存在返回错误", func(t *testing.T) {
		if err := run([]string{"proxy", "-config", filepath.Join(t.TempDir(), "nope.yaml")}); err == nil {
			t.Fatal("config 文件不存在应返回错误")
		}
	})

	t.Run("config 非法 duration 返回错误", func(t *testing.T) {
		path := writeCfg(t, "proxy:\n  dial_timeout: not-a-duration\n")
		if err := run([]string{"proxy", "-config", path}); err == nil {
			t.Fatal("非法 dial_timeout 应返回错误")
		}
	})

	t.Run("config 提供非法上游触发解析错误", func(t *testing.T) {
		path := writeCfg(t, "proxy:\n  upstream: ftp://host:21\n")
		if err := run([]string{"proxy", "-config", path}); err == nil {
			t.Fatal("配置中非法上游应触发错误")
		}
	})
}

// TestRunProxyMultiValidation 覆盖 proxy 多实例的校验路径：重复监听地址、
// 某实例非法参数、某实例非法上游，均应在启动前返回错误。
func TestRunProxyMultiValidation(t *testing.T) {
	writeCfg := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("写入临时配置失败: %v", err)
		}
		return path
	}

	t.Run("重复监听地址报错", func(t *testing.T) {
		path := writeCfg(t, "proxy:\n  - addr: 127.0.0.1:1080\n  - addr: 127.0.0.1:1080\n")
		if err := run([]string{"proxy", "-config", path}); err == nil {
			t.Fatal("重复监听地址应返回错误")
		}
	})

	t.Run("某实例负超时报错", func(t *testing.T) {
		path := writeCfg(t, "proxy:\n  - addr: 127.0.0.1:1080\n  - addr: 127.0.0.1:1081\n    dial_timeout: -1s\n")
		if err := run([]string{"proxy", "-config", path}); err == nil {
			t.Fatal("负 dial_timeout 应返回错误")
		}
	})

	t.Run("某实例非法上游报错", func(t *testing.T) {
		path := writeCfg(t, "proxy:\n  - addr: 127.0.0.1:1080\n  - addr: 127.0.0.1:1081\n    upstream: ftp://host:21\n")
		if err := run([]string{"proxy", "-config", path}); err == nil {
			t.Fatal("非法上游应返回错误")
		}
	})
}

// TestRunForwardMultiValidation 覆盖 forward 多实例的校验路径：重复监听地址、
// 某实例非法端口，均应在启动前返回错误。
func TestRunForwardMultiValidation(t *testing.T) {
	writeCfg := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("写入临时配置失败: %v", err)
		}
		return path
	}

	t.Run("重复监听地址报错", func(t *testing.T) {
		path := writeCfg(t, "forward:\n  - listen_port: 22\n    target: a:1\n  - listen_port: 22\n    target: b:2\n")
		if err := run([]string{"-config", path}); err == nil {
			t.Fatal("重复监听地址应返回错误")
		}
	})

	t.Run("某实例非法端口报错", func(t *testing.T) {
		path := writeCfg(t, "forward:\n  - listen_port: 22\n    target: a:1\n  - listen_port: 0\n    target: b:2\n")
		if err := run([]string{"-config", path}); err == nil {
			t.Fatal("非法端口应返回错误")
		}
	})

	t.Run("某实例空 target 报错", func(t *testing.T) {
		path := writeCfg(t, "forward:\n  - listen_port: 22\n    target: a:1\n  - listen_port: 23\n    target: \"\"\n")
		if err := run([]string{"-config", path}); err == nil {
			t.Fatal("空 target 应返回错误")
		}
	})

	t.Run("socat 模式不会被静默当作 go", func(t *testing.T) {
		path := writeCfg(t, "forward:\n  - listen_port: 22\n    target: a:1\n    mode: go\n  - listen_port: 23\n    target: b:2\n    mode: socat\n")
		err := run([]string{"-config", path})
		if err == nil || !strings.Contains(err.Error(), "go") {
			t.Fatalf("多实例 socat 应返回明确错误，实际: %v", err)
		}
	})
}

func TestForwardMultiInstanceIdentityAndGlobalFlags(t *testing.T) {
	t.Run("TCP 和 UDP 可复用同一地址", func(t *testing.T) {
		tcp := options{listenHost: "127.0.0.1", listenPort: 9000, proto: "tcp"}
		udp := tcp
		udp.proto = "udp"
		if forwardInstanceKey(tcp) == forwardInstanceKey(udp) {
			t.Fatal("实例标识必须包含网络类型")
		}
	})

	t.Run("全局 CLI 管理参数覆盖实例配置", func(t *testing.T) {
		opt := options{statsAddr: "127.0.0.1:1", webAddr: "127.0.0.1:2", webLogMax: 10}
		base := options{statsAddr: "127.0.0.1:11", webAddr: "127.0.0.1:12", webLogMax: 99}
		applyForwardGlobalFlags(&opt, base, map[string]bool{
			"stats-addr":  true,
			"web-addr":    true,
			"web-log-max": true,
		})
		if opt.statsAddr != base.statsAddr || opt.webAddr != base.webAddr || opt.webLogMax != base.webLogMax {
			t.Fatalf("全局 CLI 参数未应用: %+v", opt)
		}
		if !opt.webLogMaxSet {
			t.Fatal("显式 CLI web-log-max 应保留显式配置状态")
		}
	})

	t.Run("冲突的聚合端点返回错误", func(t *testing.T) {
		_, err := collectForwardAdminOptions([]options{
			{statsAddr: "127.0.0.1:9001"},
			{statsAddr: "127.0.0.1:9002"},
		})
		if err == nil {
			t.Fatal("不同 stats_addr 不应被静默取第一个")
		}
	})

	t.Run("省略日志容量不与显式值冲突", func(t *testing.T) {
		got, err := collectForwardAdminOptions([]options{
			{webAddr: "127.0.0.1:9002", webLogMax: 1000},
			{webAddr: "127.0.0.1:9002", webLogMax: 50, webLogMaxSet: true},
		})
		if err != nil {
			t.Fatalf("省略默认值不应与显式值冲突: %v", err)
		}
		if got.webLogMax != 50 {
			t.Fatalf("webLogMax = %d, want 50", got.webLogMax)
		}
	})

	t.Run("不同显式日志容量返回错误", func(t *testing.T) {
		_, err := collectForwardAdminOptions([]options{
			{webAddr: "127.0.0.1:9002", webLogMax: 50, webLogMaxSet: true},
			{webAddr: "127.0.0.1:9002", webLogMax: 100, webLogMaxSet: true},
		})
		if err == nil {
			t.Fatal("不同显式 web_log_max 不应被静默取第一个")
		}
	})

	t.Run("面板禁用时忽略日志容量冲突", func(t *testing.T) {
		_, err := collectForwardAdminOptions([]options{
			{webLogMax: 50, webLogMaxSet: true},
			{webLogMax: 100, webLogMaxSet: true},
		})
		if err != nil {
			t.Fatalf("未启用 Web 面板时不应校验 web_log_max: %v", err)
		}
	})
}

// TestStartForwardInstancesGracefulShutdown 以两个回环高位端口做集成式校验：
// 并发启动两个 forward 实例，ctx 取消后应全部优雅退出并返回 nil。
func TestStartForwardInstancesGracefulShutdown(t *testing.T) {
	p1 := freeLoopbackPort(t)
	p2 := freeLoopbackPort(t)
	instances := []options{
		{listenPort: p1, listenHost: "127.0.0.1", target: "127.0.0.1:1", proto: "tcp", reuseAddr: true, dialTimeout: time.Second, logLevel: "info", quiet: true},
		{listenPort: p2, listenHost: "127.0.0.1", target: "127.0.0.1:1", proto: "tcp", reuseAddr: true, dialTimeout: time.Second, logLevel: "info", quiet: true},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- startForwardInstances(ctx, instances) }()

	// 给实例一点时间完成监听，再触发关闭。
	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("多实例优雅关闭应返回 nil，实际 %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("多实例未在超时内退出")
	}
}

// TestStartForwardInstancesDuplicateBindFails 验证当某实例监听地址已被占用时，
// 启动会因该实例致命错误而整体返回错误（而非永久阻塞）。
func TestStartForwardInstancesDuplicateBindFails(t *testing.T) {
	// 占用一个端口，使 forward 实例绑定失败。
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("占位监听失败: %v", err)
	}
	defer func() { _ = ln.Close() }()
	busyPort := ln.Addr().(*net.TCPAddr).Port

	instances := []options{
		{listenPort: busyPort, listenHost: "127.0.0.1", target: "127.0.0.1:1", proto: "tcp", reuseAddr: false, dialTimeout: time.Second, logLevel: "info", quiet: true},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := startForwardInstances(ctx, instances); err == nil {
		t.Fatal("绑定已占用端口应返回错误")
	}
}

// freeLoopbackPort 返回一个当前可用的回环端口（先监听再关闭以获取端口号）。
func freeLoopbackPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("获取空闲端口失败: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

// TestStatsAddrFromConfigParsed 验证配置文件中的 stats_addr 会经 loadForwardConfig
// 与 mergeConfig 合并进 options（新旧布局均覆盖）。
func TestStatsAddrFromConfigParsed(t *testing.T) {
	writeCfg := func(t *testing.T, content string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("写入临时配置失败: %v", err)
		}
		return path
	}

	t.Run("平铺布局", func(t *testing.T) {
		path := writeCfg(t, "stats_addr: 127.0.0.1:9090\n")
		cfg, err := loadForwardConfig(path)
		if err != nil {
			t.Fatalf("loadForwardConfig: %v", err)
		}
		opt := options{}
		if err := mergeConfig(&opt, cfg, map[string]bool{}); err != nil {
			t.Fatalf("mergeConfig: %v", err)
		}
		if opt.statsAddr != "127.0.0.1:9090" {
			t.Fatalf("statsAddr=%q，期望 127.0.0.1:9090", opt.statsAddr)
		}
	})

	t.Run("嵌套布局", func(t *testing.T) {
		path := writeCfg(t, "forward:\n  stats_addr: 127.0.0.1:9091\n")
		cfg, err := loadForwardConfig(path)
		if err != nil {
			t.Fatalf("loadForwardConfig: %v", err)
		}
		opt := options{}
		if err := mergeConfig(&opt, cfg, map[string]bool{}); err != nil {
			t.Fatalf("mergeConfig: %v", err)
		}
		if opt.statsAddr != "127.0.0.1:9091" {
			t.Fatalf("statsAddr=%q，期望 127.0.0.1:9091", opt.statsAddr)
		}
	})
}

// TestStartForwardInstancesWithStatsEndpoint 验证多实例 forward 携带 stats-addr
// 时会并发启动统计 HTTP 端点（/stats、/metrics），并在 ctx 取消时优雅关闭。
func TestStartForwardInstancesWithStatsEndpoint(t *testing.T) {
	p1 := freeLoopbackPort(t)
	statsPort := freeLoopbackPort(t)
	statsAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(statsPort))
	instances := []options{
		{listenPort: p1, listenHost: "127.0.0.1", target: "127.0.0.1:1", proto: "tcp", reuseAddr: true, dialTimeout: time.Second, logLevel: "info", quiet: true, statsAddr: statsAddr},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- startForwardInstances(ctx, instances) }()

	// 轮询等待统计端点就绪。
	var body string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get("http://" + statsAddr + "/stats")
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			body = string(b)
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(body, "total_conns") {
		t.Fatalf("统计端点 /stats 未返回预期 JSON: %q", body)
	}

	// /metrics 应返回 Prometheus 文本。
	resp, err := http.Get("http://" + statsAddr + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	mb, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(mb), "portmap_total_connections") {
		t.Fatalf("/metrics 未返回预期 Prometheus 文本: %q", mb)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("带统计端点的实例优雅关闭应返回 nil，实际 %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("未在超时内退出")
	}
}

func TestCollectProxyAdminOptionsRejectsConflicts(t *testing.T) {
	tests := []struct {
		name      string
		instances []proxyOptions
	}{
		{
			name: "stats addresses",
			instances: []proxyOptions{
				{statsAddr: "127.0.0.1:9090"},
				{statsAddr: "127.0.0.1:9091"},
			},
		},
		{
			name: "stats public policy",
			instances: []proxyOptions{
				{statsAddr: "0.0.0.0:9090", statsAllowPublic: true},
				{statsAddr: "0.0.0.0:9090", statsAllowPublic: false},
			},
		},
		{
			name: "web addresses",
			instances: []proxyOptions{
				{webAddr: "127.0.0.1:8080", webLogMax: 1000},
				{webAddr: "127.0.0.1:8081", webLogMax: 1000},
			},
		},
		{
			name: "web public policy",
			instances: []proxyOptions{
				{webAddr: "0.0.0.0:8080", webAllowPublic: true, webLogMax: 1000},
				{webAddr: "0.0.0.0:8080", webAllowPublic: false, webLogMax: 1000},
			},
		},
		{
			name: "web log capacity",
			instances: []proxyOptions{
				{webAddr: "127.0.0.1:8080", webLogMax: 100},
				{webAddr: "127.0.0.1:8080", webLogMax: 200},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := collectProxyAdminOptions(tt.instances); err == nil {
				t.Fatal("expected conflicting admin options to be rejected")
			}
		})
	}
}

func TestCollectProxyAdminOptionsAcceptsConsistentSettings(t *testing.T) {
	instances := []proxyOptions{
		{
			statsAddr:        "127.0.0.1:9090",
			statsAllowPublic: false,
			webAddr:          "127.0.0.1:8080",
			webAllowPublic:   false,
			webLogMax:        200,
		},
		{
			statsAddr:        "127.0.0.1:9090",
			statsAllowPublic: false,
			webAddr:          "127.0.0.1:8080",
			webAllowPublic:   false,
			webLogMax:        200,
		},
	}
	got, err := collectProxyAdminOptions(instances)
	if err != nil {
		t.Fatal(err)
	}
	if got.statsAddr != "127.0.0.1:9090" || got.webAddr != "127.0.0.1:8080" || got.webLogMax != 200 {
		t.Fatalf("unexpected consolidated options: %+v", got)
	}
}
