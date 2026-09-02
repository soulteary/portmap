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

var messagesZH = map[string]string{
	KeyUsageTitle:  "portmap - TCP/UDP 端口转发 (socat 等价实现)",
	KeyUsageLine:   "用法: %s [flags]\n\n等价于: sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222\n\nflags:",
	KeyVersionLine: "portmap %s (commit %s, built %s)",

	KeyFlagListenPort:  "本地监听端口",
	KeyFlagListenHost:  "本地监听地址（默认所有网卡）",
	KeyFlagTarget:      "转发目标地址 host:port",
	KeyFlagMode:        "转发模式：go（纯 Go 实现）或 socat（调用系统 socat）",
	KeyFlagProto:       "转发协议：tcp 或 udp",
	KeyFlagReuseAddr:   "启用 SO_REUSEADDR",
	KeyFlagSudo:        "socat 模式下是否以 sudo 运行",
	KeyFlagDialTimeout: "拨号到目标的超时时间",
	KeyFlagMaxConns:    "最大并发连接数，0 表示不限制（仅 go 模式；UDP 下限制并发会话数）",
	KeyFlagIdleTimeout: "空闲超时，双向无数据则断开，0 表示不启用（仅 go 模式；UDP 下 0 表示默认 60s 回收空闲会话）",
	KeyFlagLogLevel:    "日志级别：info 或 debug（仅 go 模式）",
	KeyFlagQuiet:       "安静模式，抑制每连接的常规日志（仅 go 模式）",
	KeyFlagVersion:     "打印版本信息后退出",
	KeyFlagConfig:      "YAML 配置文件路径",
	KeyFlagLang:        "界面语言（%s）；默认自动检测系统语言",

	KeyErrListenPort:  "非法监听端口: %d",
	KeyErrTargetEmpty: "target 不能为空",
	KeyErrProto:       "未知 proto: %q（可选 tcp 或 udp）",
	KeyErrIdleNeg:     "idle-timeout 不能为负: %s",
	KeyErrMaxConnsNeg: "max-conns 不能为负: %d",
	KeyErrDialNeg:     "dial-timeout 不能为负: %s",
	KeyErrLogLevel:    "未知 log-level: %q（可选 info 或 debug）",
	KeyErrMode:        "未知 mode: %q（可选 go 或 socat）",
	KeyErrServeExit:   "转发服务退出: %w",
	KeyErrSocatFailed: "socat 执行失败: %w",

	KeyLogEffectiveConfig: "生效配置: %s",
	KeyLogSocatIgnore:     "提示: socat 模式忽略以下仅 go 模式支持的参数: %s",
	KeyLogSocatExec:       "执行: %s",
	KeyLogStatus:          "status: active=%d total=%d",

	KeyErrConfigRead:      "读取配置文件失败: %w",
	KeyErrConfigParse:     "解析配置文件失败: %w",
	KeyErrConfigDial:      "配置文件 dial_timeout 非法: %w",
	KeyErrConfigIdle:      "配置文件 idle_timeout 非法: %w",
	KeyErrConfigHandshake: "配置文件 handshake_timeout 非法: %w",
	KeyErrConfigKeepalive: "配置文件 upstream_keepalive 非法: %w",

	KeyErrUnsupportedNet:  "不支持的网络类型: %q",
	KeyLogTCPListening:    "正在监听 %s (tcp)，转发至 %s (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogDialFailed:      "拨号 %s 失败: %v",
	KeyLogConnOpen:        "[#%d] 建立 %s <-> %s (active=%d)",
	KeyLogConnClose:       "[#%d] 关闭 %s <-> %s (up=%dB down=%dB dur=%s)",
	KeyLogPipeError:       "[#%d] 转发 %s 出错: %v",
	KeyLogUDPListening:    "正在监听 %s (udp)，转发至 %s (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogUDPLimit:        "udp 会话数已达上限，丢弃来自 %s 的数据包",
	KeyLogUDPDialFailed:   "udp 拨号 %s 失败: %v",
	KeyLogUDPWriteTarget:  "udp 写入目标失败: %v",
	KeyLogUDPSessionOpen:  "[#%d] udp 会话 %s <-> %s (active=%d)",
	KeyLogUDPSessionClose: "[#%d] udp 会话已关闭 %s",
	KeyLogUDPWriteClient:  "udp 写回客户端失败: %v",

	KeyErrSocatProto:      "非法 proto: %q",
	KeyErrSocatPort:       "非法监听端口: %d",
	KeyErrSocatTarget:     "target 为空",
	KeyErrSocatNotFound:   "在 PATH 中未找到 %q: %w",
	KeyErrSocatInvalidStr: "<非法的 socat 选项>",

	KeyUsageSubcommands: "子命令:\n  forward   TCP/UDP 端口转发（默认）\n  proxy     单端口 SOCKS5 + HTTP 代理\n  version   打印版本信息",
	KeyErrUnknownSub:    "未知子命令: %q（可选 forward、proxy 或 version）",

	KeyProxyUsageTitle: "portmap proxy - 单端口 SOCKS5 + HTTP 代理",
	KeyProxyUsageLine:  "用法: %s proxy [flags]\n\n同一监听端口自动识别 SOCKS5 与 HTTP/HTTPS 客户端；出站连接默认直连，或经配置的上游（SOCKS5/HTTP/SSH）转发。始终忽略环境代理（HTTP_PROXY/HTTPS_PROXY/ALL_PROXY）。\n\nflags:",

	KeyFlagProxyAddr:             "监听地址，SOCKS5 与 HTTP 共用此端口",
	KeyFlagProxyDialTimeout:      "出站连接超时时间",
	KeyFlagProxyMaxConns:         "代理最大并发连接数，0 表示不限制",
	KeyFlagProxyHandshakeTimeout: "协议握手超时，0 表示不限制",
	KeyFlagProxyIdleTimeout:      "双向空闲超时，0 表示不限制",
	KeyFlagProxyAllowPublic:      "允许监听非回环地址（代理不提供身份认证）",

	KeyFlagProxyUpstream:           "出站连接使用的上游代理 URL，如 socks5://user:pass@host:1080、http://host:3128、ssh://user@host:22（留空表示直连）",
	KeyFlagProxyUpstreamIdentity:   "SSH 上游认证使用的私钥文件",
	KeyFlagProxyUpstreamKnownHosts: "SSH host key 校验使用的 known_hosts 文件（默认 ~/.ssh/known_hosts）",
	KeyFlagProxyUpstreamInsecure:   "跳过 SSH 上游 host key 校验（不安全，仅用于自建测试环境）",

	KeyFlagProxyUpstreamKeepalive:            "SSH 上游主动保活探测间隔（0 表示默认 30s；负数表示禁用主动保活）",
	KeyFlagProxyUpstreamKeepaliveMaxFailures: "连续保活探测失败多少次后判定 SSH 上游断线并重连（默认 3）",

	KeyLogProxyStarted:        "代理服务已启动，监听 %s（SOCKS5 + HTTP，忽略环境代理）",
	KeyLogProxyAcceptFailed:   "接受连接失败: %v",
	KeyLogProxyDetectFailed:   "探测协议失败 (%s): %v",
	KeyLogProxySOCKS5Failed:   "SOCKS5 处理出错 (%s): %v",
	KeyLogProxyHTTPFailed:     "HTTP 处理出错 (%s): %v",
	KeyLogProxySOCKS5Relay:    "SOCKS5 %s -> %s",
	KeyLogProxyHTTPConnect:    "HTTP CONNECT %s -> %s",
	KeyLogProxyHTTPPlain:      "HTTP %s %s -> %s",
	KeyLogProxyShuttingDown:   "收到退出信号，正在关闭...",
	KeyLogProxyShutdownFailed: "优雅关闭未完成: %v",
	KeyLogProxyConnLimit:      "拒绝 %s：已达到连接上限（%d）",

	KeyLogProxyUpstreamEnabled:      "已启用上游代理: %s %s",
	KeyLogProxyUpstreamInsecure:     "警告: 已禁用 SSH 上游 host key 校验（-upstream-insecure），连接易受中间人攻击",
	KeyLogProxyUpstreamSSHConnect:   "SSH 上游已连接: %s",
	KeyLogProxyUpstreamSSHReconnect: "SSH 上游连接已断开，正在重连: %s",

	KeyLogProxyUpstreamSSHKeepaliveFail: "SSH 上游保活探测失败 %[2]d/%[3]d 次: %[1]s",
	KeyLogProxyUpstreamSSHBackoff:       "将在 %s 后重连 SSH 上游",

	KeyErrProxyExit:         "代理服务异常退出: %w",
	KeyErrProxyHandshakeNeg: "handshake-timeout 不能为负数: %s",
	KeyErrProxyPublicListen: "拒绝在公网地址 %s 启动无认证代理；如确认需要，请使用 -allow-public",
	KeyErrProxySelfTarget:   "拒绝代理目标 %s：该地址解析到当前监听器",

	KeyErrProxySocksReadNMethods: "读取 NMETHODS 失败: %w",
	KeyErrProxySocksReadMethods:  "读取 METHODS 失败: %w",
	KeyErrProxySocksNoAuth:       "客户端不支持无认证方式",
	KeyErrProxySocksReplyAuth:    "回复认证方法失败: %w",
	KeyErrProxySocksReadHeader:   "读取请求头失败: %w",
	KeyErrProxySocksBadVersion:   "非法的 SOCKS 版本: %d",
	KeyErrProxySocksParseAddr:    "解析目标地址失败: %w",
	KeyErrProxySocksReadPort:     "读取端口失败: %w",
	KeyErrProxySocksBadCommand:   "不支持的命令: %d",
	KeyErrProxySocksDial:         "连接目标 %s 失败: %w",
	KeyErrProxySocksReplySuccess: "回复成功失败: %w",
	KeyErrProxySocksBadAddrType:  "不支持的地址类型: %d",

	KeyErrProxyHTTPParseRequest: "解析 HTTP 请求失败: %w",
	KeyErrProxyHTTPConnectDial:  "CONNECT 连接 %s 失败: %w",
	KeyErrProxyHTTPConnectReply: "回复 CONNECT 成功失败: %w",
	KeyErrProxyHTTPDial:         "连接 %s 失败: %w",
	KeyErrProxyHTTPForward:      "转发请求到 %s 失败: %w",
	KeyErrProxyHTTPRelayResp:    "回传响应失败: %w",

	KeyErrProxyUpstreamScheme:        "不支持的上游协议: %q（可选 socks5、http 或 ssh）",
	KeyErrProxyUpstreamParse:         "解析上游 URL 失败: %w",
	KeyErrProxyUpstreamEmptyHost:     "上游 URL 必须包含主机地址",
	KeyErrProxyUpstreamSocks5:        "创建 SOCKS5 上游拨号器失败: %w",
	KeyErrProxyUpstreamHTTPConnect:   "上游 HTTP CONNECT 到 %s 失败: %w",
	KeyErrProxyUpstreamHTTPStatus:    "上游 HTTP CONNECT 到 %s 返回异常状态: %s",
	KeyErrProxyUpstreamSSHNoAuth:     "ssh 上游需要私钥文件（-upstream-identity）或上游 URL 中的密码",
	KeyErrProxyUpstreamSSHIdentity:   "读取 SSH 私钥文件 %s 失败: %w",
	KeyErrProxyUpstreamSSHParseKey:   "解析 SSH 私钥失败: %w",
	KeyErrProxyUpstreamSSHKnownHosts: "加载 SSH known_hosts %s 失败: %w",
	KeyErrProxyUpstreamSSHDial:       "建立到 %s 的 SSH 上游连接失败: %w",
	KeyErrProxyUpstreamSSHChannel:    "打开到 %s 的 SSH 通道失败: %w",
	KeyErrProxyUpstreamClosed:        "上游拨号器已关闭",
}
