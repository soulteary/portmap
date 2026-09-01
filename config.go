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
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/soulteary/portmap/internal/i18n"
)

// Config 是顶层嵌套配置模型，按子命令分段。
//
//	forward:            # forward 子命令（端口转发）
//	  listen_port: 22
//	  target: 127.0.0.1:2222
//	proxy:              # proxy 子命令（SOCKS5/HTTP 代理）
//	  addr: 127.0.0.1:1080
//	  dial_timeout: 30s
//	  max_conns: 256
//	lang: zh            # 全局界面语言
//
// 各段均为指针：nil 表示配置文件未出现该段。
type Config struct {
	Forward *ForwardConfig `yaml:"forward"`
	Proxy   *ProxyConfig   `yaml:"proxy"`
	Lang    *string        `yaml:"lang"`
}

// ForwardConfig 是 forward 子命令的配置段，字段与 forward flag 一一对应。
// 字段均为指针：nil 表示配置文件中未出现该字段，非 nil 才参与合并（覆盖默认值）。
type ForwardConfig struct {
	ListenPort  *int    `yaml:"listen_port"`
	ListenHost  *string `yaml:"listen_host"`
	Target      *string `yaml:"target"`
	Mode        *string `yaml:"mode"`
	Proto       *string `yaml:"proto"`
	ReuseAddr   *bool   `yaml:"reuseaddr"`
	Sudo        *bool   `yaml:"sudo"`
	DialTimeout *string `yaml:"dial_timeout"`
	MaxConns    *int    `yaml:"max_conns"`
	IdleTimeout *string `yaml:"idle_timeout"`
	LogLevel    *string `yaml:"log_level"`
	Quiet       *bool   `yaml:"quiet"`
}

// ProxyConfig 是 proxy 子命令的配置段，字段与 proxy flag 一一对应。
type ProxyConfig struct {
	Addr             *string `yaml:"addr"`
	DialTimeout      *string `yaml:"dial_timeout"`
	MaxConns         *int    `yaml:"max_conns"`
	HandshakeTimeout *string `yaml:"handshake_timeout"`
	IdleTimeout      *string `yaml:"idle_timeout"`
	AllowPublic      *bool   `yaml:"allow_public"`
}

// fileConfig 与命令行 flag 一一对应，字段均为指针类型：
// nil 表示配置文件中未出现该字段，非 nil 才会参与合并（覆盖默认值）。
// 这样才能区分“未设置”与“显式设置为零值”。
//
// 它对应「旧版平铺 forward 配置」布局（顶层直接是 listen_port/target/... ），
// 为向后兼容保留：loadConfig 仍按此布局解析，loadForwardConfig 会在检测到
// 新版嵌套布局时改用 Config.Forward。
type fileConfig struct {
	ListenPort  *int    `yaml:"listen_port"`
	ListenHost  *string `yaml:"listen_host"`
	Target      *string `yaml:"target"`
	Mode        *string `yaml:"mode"`
	Proto       *string `yaml:"proto"`
	ReuseAddr   *bool   `yaml:"reuseaddr"`
	Sudo        *bool   `yaml:"sudo"`
	DialTimeout *string `yaml:"dial_timeout"`
	MaxConns    *int    `yaml:"max_conns"`
	IdleTimeout *string `yaml:"idle_timeout"`
	LogLevel    *string `yaml:"log_level"`
	Quiet       *bool   `yaml:"quiet"`
	Lang        *string `yaml:"lang"`
}

// nestedTopKeys 是新版嵌套布局的顶层字段名集合。
// lang 同时存在于新旧布局，不作为判定依据。
var nestedTopKeys = map[string]bool{
	"forward": true,
	"proxy":   true,
}

// isNestedLayout 检查 YAML 顶层是否出现 forward/proxy 等新版分段字段。
// 出现任一即视为新版嵌套布局；否则按旧版平铺 forward 布局处理。
func isNestedLayout(data []byte) bool {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return false
	}
	m := doc.Content[0]
	for i := 0; i+1 < len(m.Content); i += 2 {
		if nestedTopKeys[m.Content[i].Value] {
			return true
		}
	}
	return false
}

// decodeStrict 以 KnownFields(true) 严格解码 data 到 out，空文件视为无字段。
func decodeStrict(data []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf(i18n.T(i18n.KeyErrConfigParse), err)
	}
	return nil
}

// loadTopConfig 读取并解析配置文件为顶层嵌套 Config。
// 若文件为旧版平铺 forward 布局，则自动归一化到 Config.Forward，
// 从而兼容历史配置。
func loadTopConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrConfigRead), err)
	}

	if isNestedLayout(data) {
		var cfg Config
		if err := decodeStrict(data, &cfg); err != nil {
			return nil, err
		}
		return &cfg, nil
	}

	// 旧版平铺布局：解析为 fileConfig 后归一化到 Config.Forward。
	var flat fileConfig
	if err := decodeStrict(data, &flat); err != nil {
		return nil, err
	}
	cfg := &Config{
		Forward: &ForwardConfig{
			ListenPort:  flat.ListenPort,
			ListenHost:  flat.ListenHost,
			Target:      flat.Target,
			Mode:        flat.Mode,
			Proto:       flat.Proto,
			ReuseAddr:   flat.ReuseAddr,
			Sudo:        flat.Sudo,
			DialTimeout: flat.DialTimeout,
			MaxConns:    flat.MaxConns,
			IdleTimeout: flat.IdleTimeout,
			LogLevel:    flat.LogLevel,
			Quiet:       flat.Quiet,
		},
		Lang: flat.Lang,
	}
	return cfg, nil
}

// loadForwardConfig 为 forward 子命令加载配置，兼容新旧两种布局：
//   - 新版：读取 forward: 段与顶层 lang:；
//   - 旧版：平铺 listen_port/target/... 直接映射为 forward 配置。
//
// 返回归一化后的 *fileConfig，供既有 mergeConfig 复用。
func loadForwardConfig(path string) (*fileConfig, error) {
	cfg, err := loadTopConfig(path)
	if err != nil {
		return nil, err
	}
	out := &fileConfig{Lang: cfg.Lang}
	if cfg.Forward != nil {
		fw := cfg.Forward
		out.ListenPort = fw.ListenPort
		out.ListenHost = fw.ListenHost
		out.Target = fw.Target
		out.Mode = fw.Mode
		out.Proto = fw.Proto
		out.ReuseAddr = fw.ReuseAddr
		out.Sudo = fw.Sudo
		out.DialTimeout = fw.DialTimeout
		out.MaxConns = fw.MaxConns
		out.IdleTimeout = fw.IdleTimeout
		out.LogLevel = fw.LogLevel
		out.Quiet = fw.Quiet
	}
	return out, nil
}

// mergeProxyConfig 将配置文件中的 proxy 段合并进 proxyOptions：
// 仅当配置文件出现该字段且命令行未显式设置对应 flag 时才覆盖，
// 从而保证「命令行显式 flag > 配置文件 > 内置默认值」的优先级。
func mergeProxyConfig(opt *proxyOptions, cfg *Config, setFlags map[string]bool) error {
	if cfg == nil {
		return nil
	}
	if cfg.Lang != nil && !setFlags["lang"] {
		opt.lang = *cfg.Lang
	}
	if cfg.Proxy == nil {
		return nil
	}
	if cfg.Proxy.Addr != nil && !setFlags["addr"] {
		opt.addr = *cfg.Proxy.Addr
	}
	if cfg.Proxy.DialTimeout != nil && !setFlags["dial-timeout"] {
		d, err := time.ParseDuration(*cfg.Proxy.DialTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigDial), err)
		}
		opt.dialTimeout = d
	}
	if cfg.Proxy.MaxConns != nil && !setFlags["max-conns"] {
		opt.maxConns = *cfg.Proxy.MaxConns
	}
	if cfg.Proxy.HandshakeTimeout != nil && !setFlags["handshake-timeout"] {
		d, err := time.ParseDuration(*cfg.Proxy.HandshakeTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigHandshake), err)
		}
		opt.handshakeTimeout = d
	}
	if cfg.Proxy.IdleTimeout != nil && !setFlags["idle-timeout"] {
		d, err := time.ParseDuration(*cfg.Proxy.IdleTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigIdle), err)
		}
		opt.idleTimeout = d
	}
	if cfg.Proxy.AllowPublic != nil && !setFlags["allow-public"] {
		opt.allowPublic = *cfg.Proxy.AllowPublic
	}
	return nil
}

// loadConfig 读取并解析 YAML 配置文件（旧版平铺 forward 布局）。
// 文件不存在或解析失败时返回明确错误。
// 保留此函数以兼容既有直接调用（含单元测试）。
func loadConfig(path string) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrConfigRead), err)
	}

	var cfg fileConfig
	// KnownFields 开启后，配置文件中出现未知字段会直接报错，便于及早发现拼写错误。
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		// 空文件会返回 io.EOF，视为“无任何字段”，返回空配置而非错误。
		if errors.Is(err, io.EOF) {
			return &fileConfig{}, nil
		}
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrConfigParse), err)
	}
	return &cfg, nil
}

// mergeConfig 将配置文件中的字段合并进 opt：
// 仅当配置文件出现该字段（指针非 nil）且命令行未显式设置对应 flag 时才覆盖，
// 从而保证“命令行显式 flag > 配置文件 > 内置默认值”的优先级。
// dial_timeout / idle_timeout 为字符串，经 time.ParseDuration 解析，非法值返回错误。
func mergeConfig(opt *options, cfg *fileConfig, setFlags map[string]bool) error {
	if cfg == nil {
		return nil
	}

	if cfg.ListenPort != nil && !setFlags["listen-port"] {
		opt.listenPort = *cfg.ListenPort
	}
	if cfg.ListenHost != nil && !setFlags["listen-host"] {
		opt.listenHost = *cfg.ListenHost
	}
	if cfg.Target != nil && !setFlags["target"] {
		opt.target = *cfg.Target
	}
	if cfg.Mode != nil && !setFlags["mode"] {
		opt.mode = *cfg.Mode
	}
	if cfg.Proto != nil && !setFlags["proto"] {
		opt.proto = *cfg.Proto
	}
	if cfg.ReuseAddr != nil && !setFlags["reuseaddr"] {
		opt.reuseAddr = *cfg.ReuseAddr
	}
	if cfg.Sudo != nil && !setFlags["sudo"] {
		opt.useSudo = *cfg.Sudo
	}
	if cfg.MaxConns != nil && !setFlags["max-conns"] {
		opt.maxConns = *cfg.MaxConns
	}
	if cfg.LogLevel != nil && !setFlags["log-level"] {
		opt.logLevel = *cfg.LogLevel
	}
	if cfg.Quiet != nil && !setFlags["quiet"] {
		opt.quiet = *cfg.Quiet
	}
	if cfg.Lang != nil && !setFlags["lang"] {
		opt.lang = *cfg.Lang
	}
	if cfg.DialTimeout != nil && !setFlags["dial-timeout"] {
		d, err := time.ParseDuration(*cfg.DialTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigDial), err)
		}
		opt.dialTimeout = d
	}
	if cfg.IdleTimeout != nil && !setFlags["idle-timeout"] {
		d, err := time.ParseDuration(*cfg.IdleTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigIdle), err)
		}
		opt.idleTimeout = d
	}
	return nil
}
