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
}
