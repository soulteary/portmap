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

// forward 子命令（端口转发 + socat 回退）相关的消息 key。
const (
	// forward flag 描述。
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

	// 校验/运行错误（main.go forward 分支）。
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
	KeyLogStatusFull      = "log.status-full"

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
