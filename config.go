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
// 各段均为列表：长度为 0 表示配置文件未出现该段；长度为 1 为单实例；
// 长度大于 1 为多实例（多端口映射）。为兼容历史「单对象」写法，
// ForwardConfigList / ProxyConfigList 各自实现了 UnmarshalYAML：
// 既能解析单个映射对象（视为 1 个实例），也能解析对象列表（多实例）。
type Config struct {
	Forward ForwardConfigList `yaml:"forward"`
	Proxy   ProxyConfigList   `yaml:"proxy"`
	Lang    *string           `yaml:"lang"`
}

// ForwardConfigList 是 forward 实例列表。支持两种 YAML 写法：
//   - 单对象：forward: {listen_port: 22, ...} → 归一化为 1 个实例；
//   - 列表：  forward: [{...}, {...}]          → 多实例（多端口映射）。
type ForwardConfigList []*ForwardConfig

// ProxyConfigList 是 proxy 实例列表，写法与 ForwardConfigList 一致。
type ProxyConfigList []*ProxyConfig

// UnmarshalYAML 兼容「单对象或对象列表」两种写法，并保持对每个元素的严格
// 未知字段校验（KnownFields）。yaml.v3 的 KnownFields 作用于 Decoder，无法
// 直接经由 UnmarshalYAML 的 *yaml.Node 传递，因此这里对每个子节点先序列化
// 再用严格 Decoder 解码，从而在列表元素级别保留未知字段报错。
func (l *ForwardConfigList) UnmarshalYAML(node *yaml.Node) error {
	items, err := decodeNodeList[ForwardConfig](node)
	if err != nil {
		return err
	}
	*l = items
	return nil
}

// UnmarshalYAML 见 ForwardConfigList.UnmarshalYAML 的说明。
func (l *ProxyConfigList) UnmarshalYAML(node *yaml.Node) error {
	items, err := decodeNodeList[ProxyConfig](node)
	if err != nil {
		return err
	}
	*l = items
	return nil
}

// decodeNodeList 把一个 YAML 节点解析为 *T 列表：
//   - MappingNode → 单元素列表；
//   - SequenceNode → 多元素列表（每个元素须为 MappingNode）；
//   - 其它类型报错。
//
// 每个元素都经由严格 Decoder（KnownFields(true)）解码，保证未知字段仍报错。
func decodeNodeList[T any](node *yaml.Node) ([]*T, error) {
	switch node.Kind {
	case yaml.MappingNode:
		item, err := decodeNodeStrict[T](node)
		if err != nil {
			return nil, err
		}
		return []*T{item}, nil
	case yaml.SequenceNode:
		out := make([]*T, 0, len(node.Content))
		for _, child := range node.Content {
			item, err := decodeNodeStrict[T](child)
			if err != nil {
				return nil, err
			}
			out = append(out, item)
		}
		return out, nil
	default:
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrConfigParse), errUnexpectedNodeKind(node))
	}
}

// decodeNodeStrict 以 KnownFields(true) 严格解码单个 YAML 节点到 *T。
// 通过把节点重新序列化后再解码，从而复用 Decoder 级别的严格校验。
func decodeNodeStrict[T any](node *yaml.Node) (*T, error) {
	raw, err := yaml.Marshal(node)
	if err != nil {
		return nil, fmt.Errorf(i18n.T(i18n.KeyErrConfigParse), err)
	}
	var out T
	if err := decodeStrict(raw, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// errUnexpectedNodeKind 生成一个描述非映射/序列节点的错误。
func errUnexpectedNodeKind(node *yaml.Node) error {
	return fmt.Errorf("unexpected YAML node kind %d at line %d", node.Kind, node.Line)
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
	StatsAddr   *string `yaml:"stats_addr"`
	WebAddr     *string `yaml:"web_addr"`
	WebLogMax   *int    `yaml:"web_log_max"`
}

// ProxyConfig 是 proxy 子命令的配置段，字段与 proxy flag 一一对应。
type ProxyConfig struct {
	Addr                       *string `yaml:"addr"`
	DialTimeout                *string `yaml:"dial_timeout"`
	MaxConns                   *int    `yaml:"max_conns"`
	HandshakeTimeout           *string `yaml:"handshake_timeout"`
	IdleTimeout                *string `yaml:"idle_timeout"`
	AllowPublic                *bool   `yaml:"allow_public"`
	Upstream                   *string `yaml:"upstream"`
	UpstreamIdentity           *string `yaml:"upstream_identity"`
	UpstreamIdentityPassphrase *string `yaml:"upstream_identity_passphrase"`
	UpstreamKnownHosts         *string `yaml:"upstream_known_hosts"`
	UpstreamInsecure           *bool   `yaml:"upstream_insecure"`
	UpstreamAgent              *bool   `yaml:"upstream_agent"`
	UpstreamAgentSocket        *string `yaml:"upstream_agent_socket"`

	UpstreamKeepalive            *string `yaml:"upstream_keepalive"`
	UpstreamKeepaliveMaxFailures *int    `yaml:"upstream_keepalive_max_failures"`
	StatsAddr                    *string `yaml:"stats_addr"`
	StatsAllowPublic             *bool   `yaml:"stats_allow_public"`
	WebAddr                      *string `yaml:"web_addr"`
	WebAllowPublic               *bool   `yaml:"web_allow_public"`
	WebLogMax                    *int    `yaml:"web_log_max"`
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
	StatsAddr   *string `yaml:"stats_addr"`
	WebAddr     *string `yaml:"web_addr"`
	WebLogMax   *int    `yaml:"web_log_max"`
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
	cfg := &Config{Lang: flat.Lang}
	// An explicitly supplied but empty/irrelevant config must not silently
	// start the default forwarder. Keep Forward empty so the selected command
	// can fail closed with a clear error.
	legacy := flat
	legacy.Lang = nil
	if legacy == (fileConfig{}) {
		return cfg, nil
	}
	cfg.Forward = ForwardConfigList{{
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
		StatsAddr:   flat.StatsAddr,
		WebAddr:     flat.WebAddr,
		WebLogMax:   flat.WebLogMax,
	}}
	return cfg, nil
}

// loadForwardConfig 为 forward 子命令加载配置，兼容新旧两种布局：
//   - 新版：读取 forward: 段（单对象或列表的第一个实例）与顶层 lang:；
//   - 旧版：平铺 listen_port/target/... 直接映射为 forward 配置。
//
// 返回归一化后的 *fileConfig，供既有 mergeConfig（CLI 单实例合并）复用。
// 多实例场景不经由此函数，而是逐个 *ForwardConfig 转换为运行参数。
func loadForwardConfig(path string) (*fileConfig, error) {
	cfg, err := loadTopConfig(path)
	if err != nil {
		return nil, err
	}
	out := &fileConfig{Lang: cfg.Lang}
	if len(cfg.Forward) > 0 && cfg.Forward[0] != nil {
		fw := cfg.Forward[0]
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
		out.StatsAddr = fw.StatsAddr
		out.WebAddr = fw.WebAddr
		out.WebLogMax = fw.WebLogMax
	}
	return out, nil
}

// mergeProxyConfig 将配置文件中的 proxy 段合并进 proxyOptions：
// 仅当配置文件出现该字段且命令行未显式设置对应 flag 时才覆盖，
// 从而保证「命令行显式 flag > 配置文件 > 内置默认值」的优先级。
//
// 该函数用于 CLI 单实例合并路径：仅合并 proxy 段的「第一个/唯一」实例。
// 多实例场景由 buildProxyOptions 逐个转换，不经由此函数。
func mergeProxyConfig(opt *proxyOptions, cfg *Config, setFlags map[string]bool) error {
	if cfg == nil {
		return nil
	}
	if cfg.Lang != nil && !setFlags["lang"] {
		opt.lang = *cfg.Lang
	}
	if len(cfg.Proxy) == 0 || cfg.Proxy[0] == nil {
		return nil
	}
	return applyProxyConfig(opt, cfg.Proxy[0], setFlags)
}

// applyProxyConfig 将单个 *ProxyConfig 合并进 proxyOptions：仅覆盖配置文件中
// 出现（指针非 nil）且命令行未显式设置（setFlags 为空/未标记）的字段。
// 多实例场景调用时 setFlags 传入空 map（per-instance 不应用 CLI 覆盖）。
func applyProxyConfig(opt *proxyOptions, pc *ProxyConfig, setFlags map[string]bool) error {
	if pc == nil {
		return nil
	}
	if pc.Addr != nil && !setFlags["addr"] {
		opt.addr = *pc.Addr
	}
	if pc.DialTimeout != nil && !setFlags["dial-timeout"] {
		d, err := time.ParseDuration(*pc.DialTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigDial), err)
		}
		opt.dialTimeout = d
	}
	if pc.MaxConns != nil && !setFlags["max-conns"] {
		opt.maxConns = *pc.MaxConns
	}
	if pc.HandshakeTimeout != nil && !setFlags["handshake-timeout"] {
		d, err := time.ParseDuration(*pc.HandshakeTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigHandshake), err)
		}
		opt.handshakeTimeout = d
	}
	if pc.IdleTimeout != nil && !setFlags["idle-timeout"] {
		d, err := time.ParseDuration(*pc.IdleTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigIdle), err)
		}
		opt.idleTimeout = d
	}
	if pc.AllowPublic != nil && !setFlags["allow-public"] {
		opt.allowPublic = *pc.AllowPublic
	}
	if pc.Upstream != nil && !setFlags["upstream"] {
		opt.upstream = *pc.Upstream
	}
	if pc.UpstreamIdentity != nil && !setFlags["upstream-identity"] {
		opt.upstreamIdentity = *pc.UpstreamIdentity
	}
	// upstream_identity_passphrase 无对应命令行 flag（安全考虑），故不参与
	// setFlags 门控；环境变量优先级更高，在 resolveIdentityPassphrase 中处理。
	if pc.UpstreamIdentityPassphrase != nil {
		opt.upstreamIdentityPassphrase = *pc.UpstreamIdentityPassphrase
	}
	if pc.UpstreamKnownHosts != nil && !setFlags["upstream-known-hosts"] {
		opt.upstreamKnownHosts = *pc.UpstreamKnownHosts
	}
	if pc.UpstreamInsecure != nil && !setFlags["upstream-insecure"] {
		opt.upstreamInsecure = *pc.UpstreamInsecure
	}
	if pc.UpstreamAgent != nil && !setFlags["upstream-agent"] {
		opt.upstreamAgent = *pc.UpstreamAgent
	}
	if pc.UpstreamAgentSocket != nil && !setFlags["upstream-agent-socket"] {
		opt.upstreamAgentSocket = *pc.UpstreamAgentSocket
	}
	if pc.UpstreamKeepalive != nil && !setFlags["upstream-keepalive"] {
		d, err := time.ParseDuration(*pc.UpstreamKeepalive)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigKeepalive), err)
		}
		opt.upstreamKeepalive = d
	}
	if pc.UpstreamKeepaliveMaxFailures != nil && !setFlags["upstream-keepalive-max-failures"] {
		opt.upstreamKeepaliveMaxFailures = *pc.UpstreamKeepaliveMaxFailures
	}
	if pc.StatsAddr != nil && !setFlags["stats-addr"] {
		opt.statsAddr = *pc.StatsAddr
	}
	if pc.StatsAllowPublic != nil && !setFlags["stats-allow-public"] {
		opt.statsAllowPublic = *pc.StatsAllowPublic
	}
	if pc.WebAddr != nil && !setFlags["web-addr"] {
		opt.webAddr = *pc.WebAddr
	}
	if pc.WebAllowPublic != nil && !setFlags["web-allow-public"] {
		opt.webAllowPublic = *pc.WebAllowPublic
	}
	if pc.WebLogMax != nil && !setFlags["web-log-max"] {
		opt.webLogMax = *pc.WebLogMax
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
	if cfg.StatsAddr != nil && !setFlags["stats-addr"] {
		opt.statsAddr = *cfg.StatsAddr
	}
	if cfg.WebAddr != nil && !setFlags["web-addr"] {
		opt.webAddr = *cfg.WebAddr
	}
	if cfg.WebLogMax != nil && !setFlags["web-log-max"] {
		opt.webLogMax = *cfg.WebLogMax
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

// applyForwardConfig 将单个 *ForwardConfig 合并进 options：仅覆盖配置文件中
// 出现（指针非 nil）且命令行未显式设置的字段。多实例场景调用时 setFlags 传入
// 空 map（per-instance 不应用 CLI 覆盖）。ForwardConfig 不含 lang 字段，
// 语言由顶层 Config.Lang 统一处理。
func applyForwardConfig(opt *options, fc *ForwardConfig, setFlags map[string]bool) error {
	if fc == nil {
		return nil
	}
	if fc.ListenPort != nil && !setFlags["listen-port"] {
		opt.listenPort = *fc.ListenPort
	}
	if fc.ListenHost != nil && !setFlags["listen-host"] {
		opt.listenHost = *fc.ListenHost
	}
	if fc.Target != nil && !setFlags["target"] {
		opt.target = *fc.Target
	}
	if fc.Mode != nil && !setFlags["mode"] {
		opt.mode = *fc.Mode
	}
	if fc.Proto != nil && !setFlags["proto"] {
		opt.proto = *fc.Proto
	}
	if fc.ReuseAddr != nil && !setFlags["reuseaddr"] {
		opt.reuseAddr = *fc.ReuseAddr
	}
	if fc.Sudo != nil && !setFlags["sudo"] {
		opt.useSudo = *fc.Sudo
	}
	if fc.MaxConns != nil && !setFlags["max-conns"] {
		opt.maxConns = *fc.MaxConns
	}
	if fc.LogLevel != nil && !setFlags["log-level"] {
		opt.logLevel = *fc.LogLevel
	}
	if fc.Quiet != nil && !setFlags["quiet"] {
		opt.quiet = *fc.Quiet
	}
	if fc.StatsAddr != nil && !setFlags["stats-addr"] {
		opt.statsAddr = *fc.StatsAddr
	}
	if fc.WebAddr != nil && !setFlags["web-addr"] {
		opt.webAddr = *fc.WebAddr
	}
	if fc.WebLogMax != nil && !setFlags["web-log-max"] {
		opt.webLogMax = *fc.WebLogMax
	}
	if fc.DialTimeout != nil && !setFlags["dial-timeout"] {
		d, err := time.ParseDuration(*fc.DialTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigDial), err)
		}
		opt.dialTimeout = d
	}
	if fc.IdleTimeout != nil && !setFlags["idle-timeout"] {
		d, err := time.ParseDuration(*fc.IdleTimeout)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrConfigIdle), err)
		}
		opt.idleTimeout = d
	}
	return nil
}
