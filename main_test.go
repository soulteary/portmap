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
