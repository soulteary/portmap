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
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeTempConfig 在临时目录写入 YAML 内容并返回其路径。
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	t.Run("正常解析全部字段", func(t *testing.T) {
		path := writeTempConfig(t, `
listen_port: 8022
listen_host: 127.0.0.1
target: 127.0.0.1:2222
mode: go
proto: udp
reuseaddr: false
sudo: true
dial_timeout: 15s
max_conns: 100
idle_timeout: 5m
log_level: debug
quiet: true
`)
		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("loadConfig 返回错误: %v", err)
		}
		if cfg.ListenPort == nil || *cfg.ListenPort != 8022 {
			t.Errorf("listen_port 解析错误: %v", cfg.ListenPort)
		}
		if cfg.Proto == nil || *cfg.Proto != "udp" {
			t.Errorf("proto 解析错误: %v", cfg.Proto)
		}
		if cfg.ReuseAddr == nil || *cfg.ReuseAddr != false {
			t.Errorf("reuseaddr 解析错误: %v", cfg.ReuseAddr)
		}
		if cfg.DialTimeout == nil || *cfg.DialTimeout != "15s" {
			t.Errorf("dial_timeout 解析错误: %v", cfg.DialTimeout)
		}
		if cfg.IdleTimeout == nil || *cfg.IdleTimeout != "5m" {
			t.Errorf("idle_timeout 解析错误: %v", cfg.IdleTimeout)
		}
	})

	t.Run("字段缺省时对应指针为 nil", func(t *testing.T) {
		path := writeTempConfig(t, "listen_port: 2200\n")
		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("loadConfig 返回错误: %v", err)
		}
		if cfg.ListenPort == nil || *cfg.ListenPort != 2200 {
			t.Errorf("listen_port 解析错误: %v", cfg.ListenPort)
		}
		if cfg.Target != nil {
			t.Errorf("target 未出现应为 nil，实际: %v", *cfg.Target)
		}
		if cfg.DialTimeout != nil {
			t.Errorf("dial_timeout 未出现应为 nil，实际: %v", *cfg.DialTimeout)
		}
	})

	t.Run("空文件返回空配置", func(t *testing.T) {
		path := writeTempConfig(t, "")
		cfg, err := loadConfig(path)
		if err != nil {
			t.Fatalf("空文件不应返回错误: %v", err)
		}
		if cfg == nil || cfg.ListenPort != nil {
			t.Errorf("空文件应返回空配置，实际: %+v", cfg)
		}
	})

	t.Run("未知字段报错", func(t *testing.T) {
		path := writeTempConfig(t, "unknown_field: 1\n")
		if _, err := loadConfig(path); err == nil {
			t.Fatal("未知字段应返回错误，但未返回")
		}
	})

	t.Run("非法 YAML 报错", func(t *testing.T) {
		path := writeTempConfig(t, "listen_port: : :\n")
		if _, err := loadConfig(path); err == nil {
			t.Fatal("非法 YAML 应返回错误，但未返回")
		}
	})

	t.Run("文件不存在报错", func(t *testing.T) {
		if _, err := loadConfig(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
			t.Fatal("文件不存在应返回错误，但未返回")
		}
	})
}

func TestMergeConfig(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }
	boolPtr := func(b bool) *bool { return &b }

	t.Run("配置文件覆盖未显式设置的字段", func(t *testing.T) {
		opt := options{listenPort: 22, target: "127.0.0.1:2222", dialTimeout: 10 * time.Second}
		cfg := &fileConfig{
			ListenPort:  intPtr(9000),
			Target:      strPtr("10.0.0.1:3333"),
			DialTimeout: strPtr("30s"),
		}
		if err := mergeConfig(&opt, cfg, map[string]bool{}); err != nil {
			t.Fatalf("mergeConfig 返回错误: %v", err)
		}
		if opt.listenPort != 9000 {
			t.Errorf("listenPort 期望 9000，实际 %d", opt.listenPort)
		}
		if opt.target != "10.0.0.1:3333" {
			t.Errorf("target 期望 10.0.0.1:3333，实际 %s", opt.target)
		}
		if opt.dialTimeout != 30*time.Second {
			t.Errorf("dialTimeout 期望 30s，实际 %s", opt.dialTimeout)
		}
	})

	t.Run("命令行显式设置的字段优先于配置文件", func(t *testing.T) {
		opt := options{listenPort: 8080}
		cfg := &fileConfig{ListenPort: intPtr(9000)}
		setFlags := map[string]bool{"listen-port": true}
		if err := mergeConfig(&opt, cfg, setFlags); err != nil {
			t.Fatalf("mergeConfig 返回错误: %v", err)
		}
		if opt.listenPort != 8080 {
			t.Errorf("命令行应优先，listenPort 期望 8080，实际 %d", opt.listenPort)
		}
	})

	t.Run("非法 duration 返回错误", func(t *testing.T) {
		opt := options{}
		cfg := &fileConfig{DialTimeout: strPtr("abc")}
		if err := mergeConfig(&opt, cfg, map[string]bool{}); err == nil {
			t.Fatal("非法 dial_timeout 应返回错误，但未返回")
		}
	})

	t.Run("零值 bool 也能被配置文件覆盖", func(t *testing.T) {
		opt := options{reuseAddr: true}
		cfg := &fileConfig{ReuseAddr: boolPtr(false)}
		if err := mergeConfig(&opt, cfg, map[string]bool{}); err != nil {
			t.Fatalf("mergeConfig 返回错误: %v", err)
		}
		if opt.reuseAddr != false {
			t.Error("reuseAddr 应被配置文件覆盖为 false")
		}
	})

	t.Run("配置文件的 lang 在命令行未设置时生效", func(t *testing.T) {
		opt := options{}
		cfg := &fileConfig{Lang: strPtr("zh")}
		if err := mergeConfig(&opt, cfg, map[string]bool{}); err != nil {
			t.Fatalf("mergeConfig 返回错误: %v", err)
		}
		if opt.lang != "zh" {
			t.Errorf("lang 期望 zh，实际 %s", opt.lang)
		}
	})

	t.Run("命令行显式 lang 优先于配置文件", func(t *testing.T) {
		opt := options{lang: "en"}
		cfg := &fileConfig{Lang: strPtr("zh")}
		setFlags := map[string]bool{"lang": true}
		if err := mergeConfig(&opt, cfg, setFlags); err != nil {
			t.Fatalf("mergeConfig 返回错误: %v", err)
		}
		if opt.lang != "en" {
			t.Errorf("命令行应优先，lang 期望 en，实际 %s", opt.lang)
		}
	})

	t.Run("全部字段均可被配置文件覆盖", func(t *testing.T) {
		opt := options{}
		cfg := &fileConfig{
			ListenPort:  intPtr(2200),
			ListenHost:  strPtr("0.0.0.0"),
			Target:      strPtr("10.1.1.1:22"),
			Mode:        strPtr("socat"),
			Proto:       strPtr("udp"),
			ReuseAddr:   boolPtr(true),
			Sudo:        boolPtr(true),
			DialTimeout: strPtr("15s"),
			MaxConns:    intPtr(128),
			IdleTimeout: strPtr("90s"),
			LogLevel:    strPtr("debug"),
			Quiet:       boolPtr(true),
			Lang:        strPtr("de"),
		}
		if err := mergeConfig(&opt, cfg, map[string]bool{}); err != nil {
			t.Fatalf("mergeConfig 返回错误: %v", err)
		}
		want := options{
			listenPort:  2200,
			listenHost:  "0.0.0.0",
			target:      "10.1.1.1:22",
			mode:        "socat",
			proto:       "udp",
			reuseAddr:   true,
			useSudo:     true,
			dialTimeout: 15 * time.Second,
			maxConns:    128,
			idleTimeout: 90 * time.Second,
			logLevel:    "debug",
			quiet:       true,
			lang:        "de",
		}
		if opt != want {
			t.Fatalf("合并结果=%+v，期望 %+v", opt, want)
		}
	})

	t.Run("非法 idle_timeout 返回错误", func(t *testing.T) {
		opt := options{}
		cfg := &fileConfig{IdleTimeout: strPtr("nope")}
		if err := mergeConfig(&opt, cfg, map[string]bool{}); err == nil {
			t.Fatal("非法 idle_timeout 应返回错误")
		}
	})

	t.Run("nil 配置安全返回", func(t *testing.T) {
		opt := options{listenPort: 22}
		if err := mergeConfig(&opt, nil, map[string]bool{}); err != nil {
			t.Fatalf("nil 配置不应报错: %v", err)
		}
		if opt.listenPort != 22 {
			t.Error("nil 配置不应修改字段")
		}
	})
}

func TestMergeProxyConfigSecurityLimits(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }
	boolPtr := func(b bool) *bool { return &b }

	opt := proxyOptions{}
	cfg := &Config{Proxy: ProxyConfigList{{
		MaxConns:         intPtr(64),
		HandshakeTimeout: strPtr("3s"),
		IdleTimeout:      strPtr("2m"),
		AllowPublic:      boolPtr(true),
	}}}
	if err := mergeProxyConfig(&opt, cfg, map[string]bool{}); err != nil {
		t.Fatalf("mergeProxyConfig 返回错误: %v", err)
	}
	if opt.maxConns != 64 || opt.handshakeTimeout != 3*time.Second || opt.idleTimeout != 2*time.Minute || !opt.allowPublic {
		t.Fatalf("代理安全配置未正确合并: %+v", opt)
	}

	bad := &Config{Proxy: ProxyConfigList{{HandshakeTimeout: strPtr("invalid")}}}
	if err := mergeProxyConfig(&proxyOptions{}, bad, map[string]bool{}); err == nil {
		t.Fatal("非法 handshake_timeout 应返回错误")
	}
}

// TestIsNestedLayout 覆盖新版嵌套布局与旧版平铺布局的判定。
func TestIsNestedLayout(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want bool
	}{
		{"含 forward 段", "forward:\n  listen_port: 22\n", true},
		{"含 proxy 段", "proxy:\n  addr: 127.0.0.1:1080\n", true},
		{"旧版平铺", "listen_port: 22\ntarget: x\n", false},
		{"仅 lang 不算嵌套", "lang: zh\n", false},
		{"空文件", "", false},
		{"非法 YAML", "a: : :\n", false},
		{"顶层为标量", "hello\n", false},
	}
	for _, c := range cases {
		if got := isNestedLayout([]byte(c.yaml)); got != c.want {
			t.Errorf("%s: isNestedLayout=%v，期望 %v", c.name, got, c.want)
		}
	}
}

// TestLoadTopConfigNested 验证新版嵌套布局能正确解析 forward/proxy/lang 段。
func TestLoadTopConfigNested(t *testing.T) {
	path := writeTempConfig(t, `
lang: ja
forward:
  listen_port: 8022
  target: 127.0.0.1:2222
proxy:
  addr: 127.0.0.1:1080
  dial_timeout: 15s
  upstream: socks5://127.0.0.1:1080
`)
	cfg, err := loadTopConfig(path)
	if err != nil {
		t.Fatalf("loadTopConfig 返回错误: %v", err)
	}
	if cfg.Lang == nil || *cfg.Lang != "ja" {
		t.Errorf("lang 解析错误: %v", cfg.Lang)
	}
	if len(cfg.Forward) != 1 || cfg.Forward[0].ListenPort == nil || *cfg.Forward[0].ListenPort != 8022 {
		t.Errorf("forward.listen_port 解析错误: %+v", cfg.Forward)
	}
	if len(cfg.Proxy) != 1 || cfg.Proxy[0].Addr == nil || *cfg.Proxy[0].Addr != "127.0.0.1:1080" {
		t.Errorf("proxy.addr 解析错误: %+v", cfg.Proxy)
	}
	if cfg.Proxy[0].Upstream == nil || *cfg.Proxy[0].Upstream != "socks5://127.0.0.1:1080" {
		t.Errorf("proxy.upstream 解析错误: %+v", cfg.Proxy)
	}
}

// TestLoadTopConfigFlatNormalized 验证旧版平铺布局归一化到 Config.Forward。
func TestLoadTopConfigFlatNormalized(t *testing.T) {
	path := writeTempConfig(t, "listen_port: 2200\ntarget: 10.0.0.1:22\nlang: zh\n")
	cfg, err := loadTopConfig(path)
	if err != nil {
		t.Fatalf("loadTopConfig 返回错误: %v", err)
	}
	if len(cfg.Forward) != 1 || cfg.Forward[0].ListenPort == nil || *cfg.Forward[0].ListenPort != 2200 {
		t.Fatalf("旧版平铺未归一化到 forward: %+v", cfg.Forward)
	}
	if cfg.Lang == nil || *cfg.Lang != "zh" {
		t.Fatalf("lang 归一化错误: %v", cfg.Lang)
	}
	if len(cfg.Proxy) != 0 {
		t.Fatalf("旧版布局不应产生 proxy 段: %+v", cfg.Proxy)
	}
}

// TestLoadTopConfigErrors 覆盖文件不存在、未知字段两类错误路径。
func TestLoadTopConfigErrors(t *testing.T) {
	if _, err := loadTopConfig(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("文件不存在应返回错误")
	}
	path := writeTempConfig(t, "forward:\n  unknown_field: 1\n")
	if _, err := loadTopConfig(path); err == nil {
		t.Fatal("嵌套布局未知字段应返回错误")
	}
}

// TestLoadForwardConfigBothLayouts 验证 loadForwardConfig 对新旧布局均归一化为 fileConfig。
func TestLoadForwardConfigBothLayouts(t *testing.T) {
	t.Run("新版嵌套", func(t *testing.T) {
		path := writeTempConfig(t, "forward:\n  listen_port: 9001\n  proto: udp\nlang: fr\n")
		fc, err := loadForwardConfig(path)
		if err != nil {
			t.Fatalf("loadForwardConfig: %v", err)
		}
		if fc.ListenPort == nil || *fc.ListenPort != 9001 {
			t.Errorf("listen_port 归一化错误: %v", fc.ListenPort)
		}
		if fc.Proto == nil || *fc.Proto != "udp" {
			t.Errorf("proto 归一化错误: %v", fc.Proto)
		}
		if fc.Lang == nil || *fc.Lang != "fr" {
			t.Errorf("lang 归一化错误: %v", fc.Lang)
		}
	})
	t.Run("旧版平铺", func(t *testing.T) {
		path := writeTempConfig(t, "listen_port: 9002\ntarget: t:1\n")
		fc, err := loadForwardConfig(path)
		if err != nil {
			t.Fatalf("loadForwardConfig: %v", err)
		}
		if fc.ListenPort == nil || *fc.ListenPort != 9002 {
			t.Errorf("listen_port 归一化错误: %v", fc.ListenPort)
		}
	})
	t.Run("仅 proxy 段时 forward 字段为空", func(t *testing.T) {
		path := writeTempConfig(t, "proxy:\n  addr: 127.0.0.1:1080\n")
		fc, err := loadForwardConfig(path)
		if err != nil {
			t.Fatalf("loadForwardConfig: %v", err)
		}
		if fc.ListenPort != nil {
			t.Errorf("无 forward 段时 listen_port 应为 nil，实际 %v", *fc.ListenPort)
		}
	})
	t.Run("文件不存在返回错误", func(t *testing.T) {
		if _, err := loadForwardConfig(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
			t.Fatal("文件不存在应返回错误")
		}
	})
}

// TestMergeProxyConfigAllFields 覆盖 mergeProxyConfig 中此前未测的字段：
// upstream 相关、keepalive、以及 flag 优先与非法 duration 分支。
func TestMergeProxyConfigAllFields(t *testing.T) {
	strPtr := func(s string) *string { return &s }
	intPtr := func(i int) *int { return &i }
	boolPtr := func(b bool) *bool { return &b }

	t.Run("全部上游字段合并", func(t *testing.T) {
		opt := proxyOptions{}
		cfg := &Config{
			Lang: strPtr("de"),
			Proxy: ProxyConfigList{{
				Addr:                         strPtr("0.0.0.0:1080"),
				DialTimeout:                  strPtr("20s"),
				Upstream:                     strPtr("ssh://root@host:22"),
				UpstreamIdentity:             strPtr("/tmp/id_rsa"),
				UpstreamKnownHosts:           strPtr("/tmp/known_hosts"),
				UpstreamInsecure:             boolPtr(true),
				UpstreamKeepalive:            strPtr("45s"),
				UpstreamKeepaliveMaxFailures: intPtr(5),
			}},
		}
		if err := mergeProxyConfig(&opt, cfg, map[string]bool{}); err != nil {
			t.Fatalf("mergeProxyConfig: %v", err)
		}
		if opt.lang != "de" || opt.addr != "0.0.0.0:1080" || opt.dialTimeout != 20*time.Second {
			t.Fatalf("基础字段合并错误: %+v", opt)
		}
		if opt.upstream != "ssh://root@host:22" || opt.upstreamIdentity != "/tmp/id_rsa" ||
			opt.upstreamKnownHosts != "/tmp/known_hosts" || !opt.upstreamInsecure {
			t.Fatalf("上游字段合并错误: %+v", opt)
		}
		if opt.upstreamKeepalive != 45*time.Second || opt.upstreamKeepaliveMaxFailures != 5 {
			t.Fatalf("keepalive 字段合并错误: %+v", opt)
		}
	})

	t.Run("命令行 flag 优先于配置文件", func(t *testing.T) {
		opt := proxyOptions{addr: "127.0.0.1:9999"}
		cfg := &Config{Proxy: ProxyConfigList{{Addr: strPtr("0.0.0.0:1080")}}}
		setFlags := map[string]bool{"addr": true}
		if err := mergeProxyConfig(&opt, cfg, setFlags); err != nil {
			t.Fatalf("mergeProxyConfig: %v", err)
		}
		if opt.addr != "127.0.0.1:9999" {
			t.Fatalf("命令行应优先，addr=%q", opt.addr)
		}
	})

	t.Run("非法 dial_timeout 返回错误", func(t *testing.T) {
		cfg := &Config{Proxy: ProxyConfigList{{DialTimeout: strPtr("bad")}}}
		if err := mergeProxyConfig(&proxyOptions{}, cfg, map[string]bool{}); err == nil {
			t.Fatal("非法 dial_timeout 应返回错误")
		}
	})

	t.Run("非法 idle_timeout 返回错误", func(t *testing.T) {
		cfg := &Config{Proxy: ProxyConfigList{{IdleTimeout: strPtr("bad")}}}
		if err := mergeProxyConfig(&proxyOptions{}, cfg, map[string]bool{}); err == nil {
			t.Fatal("非法 idle_timeout 应返回错误")
		}
	})

	t.Run("非法 keepalive 返回错误", func(t *testing.T) {
		cfg := &Config{Proxy: ProxyConfigList{{UpstreamKeepalive: strPtr("bad")}}}
		if err := mergeProxyConfig(&proxyOptions{}, cfg, map[string]bool{}); err == nil {
			t.Fatal("非法 upstream_keepalive 应返回错误")
		}
	})

	t.Run("nil Config 与 nil Proxy 段安全返回", func(t *testing.T) {
		if err := mergeProxyConfig(&proxyOptions{}, nil, map[string]bool{}); err != nil {
			t.Fatalf("nil Config 不应报错: %v", err)
		}
		if err := mergeProxyConfig(&proxyOptions{}, &Config{}, map[string]bool{}); err != nil {
			t.Fatalf("nil Proxy 段不应报错: %v", err)
		}
	})
}

// TestConfigListSingleObject 验证 forward:/proxy: 写成单个对象时归一化为 1 个实例。
func TestConfigListSingleObject(t *testing.T) {
	path := writeTempConfig(t, `
forward:
  listen_port: 22
  target: 127.0.0.1:2222
proxy:
  addr: 127.0.0.1:1080
`)
	cfg, err := loadTopConfig(path)
	if err != nil {
		t.Fatalf("loadTopConfig: %v", err)
	}
	if len(cfg.Forward) != 1 || cfg.Forward[0].ListenPort == nil || *cfg.Forward[0].ListenPort != 22 {
		t.Fatalf("forward 单对象未归一化为 1 实例: %+v", cfg.Forward)
	}
	if len(cfg.Proxy) != 1 || cfg.Proxy[0].Addr == nil || *cfg.Proxy[0].Addr != "127.0.0.1:1080" {
		t.Fatalf("proxy 单对象未归一化为 1 实例: %+v", cfg.Proxy)
	}
}

// TestConfigListMultiInstance 验证 forward:/proxy: 写成对象列表时解析为多实例。
func TestConfigListMultiInstance(t *testing.T) {
	path := writeTempConfig(t, `
forward:
  - listen_port: 22
    target: 127.0.0.1:2222
  - listen_port: 80
    target: 127.0.0.1:8080
proxy:
  - addr: 0.0.0.0:8118
    allow_public: true
    upstream: socks5://127.0.0.1:1080
  - addr: 127.0.0.1:1080
`)
	cfg, err := loadTopConfig(path)
	if err != nil {
		t.Fatalf("loadTopConfig: %v", err)
	}
	if len(cfg.Forward) != 2 {
		t.Fatalf("forward 应有 2 个实例，实际 %d", len(cfg.Forward))
	}
	if *cfg.Forward[0].ListenPort != 22 || *cfg.Forward[1].ListenPort != 80 {
		t.Fatalf("forward 实例端口解析错误: %+v %+v", cfg.Forward[0], cfg.Forward[1])
	}
	if len(cfg.Proxy) != 2 {
		t.Fatalf("proxy 应有 2 个实例，实际 %d", len(cfg.Proxy))
	}
	if *cfg.Proxy[0].Addr != "0.0.0.0:8118" || cfg.Proxy[0].AllowPublic == nil || !*cfg.Proxy[0].AllowPublic {
		t.Fatalf("proxy[0] 解析错误: %+v", cfg.Proxy[0])
	}
	if *cfg.Proxy[1].Addr != "127.0.0.1:1080" {
		t.Fatalf("proxy[1] 解析错误: %+v", cfg.Proxy[1])
	}
}

// TestConfigListStrictUnknownField 验证列表元素中的未知字段仍触发严格校验报错。
func TestConfigListStrictUnknownField(t *testing.T) {
	t.Run("forward 列表未知字段", func(t *testing.T) {
		path := writeTempConfig(t, "forward:\n  - listen_port: 22\n    bogus: 1\n")
		if _, err := loadTopConfig(path); err == nil {
			t.Fatal("列表元素未知字段应返回错误")
		}
	})
	t.Run("proxy 列表未知字段", func(t *testing.T) {
		path := writeTempConfig(t, "proxy:\n  - addr: 127.0.0.1:1080\n    bogus: 1\n")
		if _, err := loadTopConfig(path); err == nil {
			t.Fatal("列表元素未知字段应返回错误")
		}
	})
	t.Run("单对象未知字段", func(t *testing.T) {
		path := writeTempConfig(t, "proxy:\n  addr: 127.0.0.1:1080\n  bogus: 1\n")
		if _, err := loadTopConfig(path); err == nil {
			t.Fatal("单对象未知字段应返回错误")
		}
	})
}

// TestConfigListScalarRejected 验证 forward:/proxy: 为标量（既非对象也非列表）时报错。
func TestConfigListScalarRejected(t *testing.T) {
	path := writeTempConfig(t, "proxy: just-a-string\n")
	if _, err := loadTopConfig(path); err == nil {
		t.Fatal("标量 proxy 段应返回错误")
	}
}

// TestLoadForwardConfigUsesFirstInstance 验证 loadForwardConfig 对多实例列表取第一个实例，
// 从而使单实例 CLI 路径仍可复用（多实例会走独立的 runForwardMulti 路径）。
func TestLoadForwardConfigUsesFirstInstance(t *testing.T) {
	path := writeTempConfig(t, "forward:\n  - listen_port: 2201\n    target: a:1\n  - listen_port: 2202\n    target: b:2\n")
	fc, err := loadForwardConfig(path)
	if err != nil {
		t.Fatalf("loadForwardConfig: %v", err)
	}
	if fc.ListenPort == nil || *fc.ListenPort != 2201 {
		t.Fatalf("应取第一个实例，listen_port=%v", fc.ListenPort)
	}
}
