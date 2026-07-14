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
)

// fileConfig 与命令行 flag 一一对应，字段均为指针类型：
// nil 表示配置文件中未出现该字段，非 nil 才会参与合并（覆盖默认值）。
// 这样才能区分“未设置”与“显式设置为零值”。
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
}

// loadConfig 读取并解析 YAML 配置文件。
// 文件不存在或解析失败时返回明确错误。
func loadConfig(path string) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
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
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
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
	if cfg.DialTimeout != nil && !setFlags["dial-timeout"] {
		d, err := time.ParseDuration(*cfg.DialTimeout)
		if err != nil {
			return fmt.Errorf("配置文件 dial_timeout 非法: %w", err)
		}
		opt.dialTimeout = d
	}
	if cfg.IdleTimeout != nil && !setFlags["idle-timeout"] {
		d, err := time.ParseDuration(*cfg.IdleTimeout)
		if err != nil {
			return fmt.Errorf("配置文件 idle_timeout 非法: %w", err)
		}
		opt.idleTimeout = d
	}
	return nil
}
