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

package i18n

// 本文件汇总跨子命令共享的消息 key，以及语言表 messages 的装配。
// 含 %-占位符的文本用于 fmt 格式化，各语言必须保持占位符顺序一致。
//
// 消息 key 常量按域拆分到多个文件：
//   - keys_common.go  ：跨命令共享（用法/版本/通用 flag/配置文件）
//   - keys_forward.go ：forward 子命令（端口转发 + socat）
//   - keys_proxy.go   ：proxy 子命令（SOCKS5/HTTP 代理）
const (
	// CLI 用法/帮助/版本。
	KeyUsageTitle  = "usage.title"
	KeyUsageLine   = "usage.line"
	KeyVersionLine = "version.line"

	// 子命令分发相关（main.go）。
	KeyUsageSubcommands = "usage.subcommands"
	KeyErrUnknownSub    = "err.unknown-sub"

	// 通用 flag 描述（各子命令共享）。
	KeyFlagVersion          = "flag.version"
	KeyFlagConfig           = "flag.config"
	KeyFlagLang             = "flag.lang"
	KeyFlagStatsAddr        = "flag.stats-addr"
	KeyFlagStatsAllowPublic = "flag.stats-allow-public"

	// 统计 HTTP 端点（stats.http）相关。
	KeyLogStatsHTTPStarted = "log.stats-http-started"
	KeyLogStatsHTTPStopped = "log.stats-http-stopped"
	KeyErrStatsHTTPServe   = "err.stats-http-serve"
	KeyErrStatsHTTPPublic  = "err.stats-http-public"

	// Web 面板（web）相关：flag 描述、运行日志与错误。
	KeyFlagWebAddr        = "flag.web-addr"
	KeyFlagWebAllowPublic = "flag.web-allow-public"
	KeyFlagWebLogMax      = "flag.web-log-max"
	KeyLogWebStarted      = "log.web-started"
	KeyLogWebStopped      = "log.web-stopped"
	KeyErrWebServe        = "err.web-serve"
	KeyErrWebPublic       = "err.web-public"

	// Web 面板页面 UI 文本（由服务端注入到 HTML 页面）。
	KeyWebTitle         = "web.title"
	KeyWebPerfSection   = "web.perf-section"
	KeyWebLogsSection   = "web.logs-section"
	KeyWebActiveConns   = "web.active-conns"
	KeyWebTotalConns    = "web.total-conns"
	KeyWebRejectedConns = "web.rejected-conns"
	KeyWebDialErrors    = "web.dial-errors"
	KeyWebUpBytes       = "web.up-bytes"
	KeyWebDownBytes     = "web.down-bytes"
	KeyWebUptime        = "web.uptime"
	KeyWebColTime       = "web.col-time"
	KeyWebColKind       = "web.col-kind"
	KeyWebColProto      = "web.col-proto"
	KeyWebColClient     = "web.col-client"
	KeyWebColTarget     = "web.col-target"
	KeyWebColUp         = "web.col-up"
	KeyWebColDown       = "web.col-down"
	KeyWebColDuration   = "web.col-duration"
	KeyWebBtnRefresh    = "web.btn-refresh"
	KeyWebAutoRefresh   = "web.auto-refresh"
	KeyWebCountUnit     = "web.count-unit"
	KeyWebEmpty         = "web.empty"

	// 配置文件错误（config.go）。
	KeyErrConfigRead      = "err.config-read"
	KeyErrConfigParse     = "err.config-parse"
	KeyErrConfigDial      = "err.config-dial"
	KeyErrConfigIdle      = "err.config-idle"
	KeyErrConfigHandshake = "err.config-handshake"
	KeyErrConfigKeepalive = "err.config-keepalive"

	// 多实例（多端口映射）相关（main.go）。
	KeyLogForwardStartingInstances = "log.forward-starting-instances"
	KeyLogProxyStartingInstances   = "log.proxy-starting-instances"
	KeyLogInstanceStarting         = "log.instance-starting"
	KeyErrInstanceFailed           = "err.instance-failed"
	KeyErrDuplicateListen          = "err.duplicate-listen"
	KeyErrConflictingMultiOption   = "err.conflicting-multi-option"
	KeyErrMultiForwardMode         = "err.multi-forward-mode"
	KeyLogMultiIgnoreFlags         = "log.multi-ignore-flags"
)

// messages 保存每种语言下 key -> 文本 的映射。
// English 为回退语言，必须包含全部 key。
var messages = map[Lang]map[string]string{
	English:  messagesEN,
	Chinese:  messagesZH,
	Japanese: messagesJA,
	Korean:   messagesKO,
	French:   messagesFR,
	German:   messagesDE,
}
