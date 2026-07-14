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

// 消息 key 常量。所有面向用户的字符串都通过这些 key 经 T() 输出。
// 含 %-占位符的文本用于 fmt 格式化，各语言必须保持占位符顺序一致。
const (
	// CLI 用法/帮助/版本。
	KeyUsageTitle  = "usage.title"
	KeyUsageLine   = "usage.line"
	KeyVersionLine = "version.line"

	// flag 描述。
	KeyFlagListenPort  = "flag.listen-port"
	KeyFlagListenHost  = "flag.listen-host"
	KeyFlagTarget      = "flag.target"
	KeyFlagMode        = "flag.mode"
	KeyFlagProto       = "flag.proto"
	KeyFlagReuseAddr   = "flag.reuseaddr"
	KeyFlagSudo        = "flag.sudo"
	KeyFlagDialTimeout = "flag.dial-timeout"
	KeyFlagMaxConns    = "flag.max-conns"
	KeyFlagIdleTimeout = "flag.idle-timeout"
	KeyFlagLogLevel    = "flag.log-level"
	KeyFlagQuiet       = "flag.quiet"
	KeyFlagVersion     = "flag.version"
	KeyFlagConfig      = "flag.config"
	KeyFlagLang        = "flag.lang"

	// 校验/运行错误（main.go）。
	KeyErrListenPort  = "err.listen-port"
	KeyErrTargetEmpty = "err.target-empty"
	KeyErrProto       = "err.proto"
	KeyErrIdleNeg     = "err.idle-neg"
	KeyErrMaxConnsNeg = "err.maxconns-neg"
	KeyErrDialNeg     = "err.dial-neg"
	KeyErrLogLevel    = "err.log-level"
	KeyErrMode        = "err.mode"
	KeyErrServeExit   = "err.serve-exit"
	KeyErrSocatFailed = "err.socat-failed"

	// 运行时日志（main.go / signals）。
	KeyLogEffectiveConfig = "log.effective-config"
	KeyLogSocatIgnore     = "log.socat-ignore"
	KeyLogSocatExec       = "log.socat-exec"
	KeyLogStatus          = "log.status"

	// 配置文件错误（config.go）。
	KeyErrConfigRead  = "err.config-read"
	KeyErrConfigParse = "err.config-parse"
	KeyErrConfigDial  = "err.config-dial"
	KeyErrConfigIdle  = "err.config-idle"

	// forward 包日志/错误。
	KeyErrUnsupportedNet  = "err.unsupported-net"
	KeyLogTCPListening    = "fwd.tcp-listening"
	KeyLogDialFailed      = "fwd.dial-failed"
	KeyLogConnOpen        = "fwd.conn-open"
	KeyLogConnClose       = "fwd.conn-close"
	KeyLogPipeError       = "fwd.pipe-error"
	KeyLogUDPListening    = "fwd.udp-listening"
	KeyLogUDPLimit        = "fwd.udp-limit"
	KeyLogUDPDialFailed   = "fwd.udp-dial-failed"
	KeyLogUDPWriteTarget  = "fwd.udp-write-target"
	KeyLogUDPSessionOpen  = "fwd.udp-session-open"
	KeyLogUDPSessionClose = "fwd.udp-session-close"
	KeyLogUDPWriteClient  = "fwd.udp-write-client"

	// socat 包错误。
	KeyErrSocatProto      = "err.socat-proto"
	KeyErrSocatPort       = "err.socat-port"
	KeyErrSocatTarget     = "err.socat-target"
	KeyErrSocatNotFound   = "err.socat-not-found"
	KeyErrSocatInvalidStr = "err.socat-invalid-str"
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
