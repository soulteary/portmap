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
