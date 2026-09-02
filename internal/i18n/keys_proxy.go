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

// proxy 子命令（SOCKS5/HTTP 应用层代理）相关的消息 key。
const (
	// proxy 用法/帮助。
	KeyProxyUsageTitle = "proxy.usage.title"
	KeyProxyUsageLine  = "proxy.usage.line"

	// proxy flag 描述。
	KeyFlagProxyAddr             = "flag.proxy-addr"
	KeyFlagProxyDialTimeout      = "flag.proxy-dial-timeout"
	KeyFlagProxyMaxConns         = "flag.proxy-max-conns"
	KeyFlagProxyHandshakeTimeout = "flag.proxy-handshake-timeout"
	KeyFlagProxyIdleTimeout      = "flag.proxy-idle-timeout"
	KeyFlagProxyAllowPublic      = "flag.proxy-allow-public"

	// proxy 上游代理链 flag 描述。
	KeyFlagProxyUpstream           = "flag.proxy-upstream"
	KeyFlagProxyUpstreamIdentity   = "flag.proxy-upstream-identity"
	KeyFlagProxyUpstreamKnownHosts = "flag.proxy-upstream-known-hosts"
	KeyFlagProxyUpstreamInsecure   = "flag.proxy-upstream-insecure"

	// proxy 运行时日志（面向用户）。
	KeyLogProxyStarted        = "proxy.started"
	KeyLogProxyAcceptFailed   = "proxy.accept-failed"
	KeyLogProxyDetectFailed   = "proxy.detect-failed"
	KeyLogProxySOCKS5Failed   = "proxy.socks5-failed"
	KeyLogProxyHTTPFailed     = "proxy.http-failed"
	KeyLogProxySOCKS5Relay    = "proxy.socks5-relay"
	KeyLogProxyHTTPConnect    = "proxy.http-connect"
	KeyLogProxyHTTPPlain      = "proxy.http-plain"
	KeyLogProxyShuttingDown   = "proxy.shutting-down"
	KeyLogProxyShutdownFailed = "proxy.shutdown-failed"
	KeyLogProxyConnLimit      = "proxy.conn-limit"

	// proxy 上游代理链运行时日志。
	KeyLogProxyUpstreamEnabled      = "proxy.upstream-enabled"
	KeyLogProxyUpstreamInsecure     = "proxy.upstream-insecure"
	KeyLogProxyUpstreamSSHConnect   = "proxy.upstream-ssh-connect"
	KeyLogProxyUpstreamSSHReconnect = "proxy.upstream-ssh-reconnect"

	// proxy 运行/退出错误（main.go proxy 分支）。
	KeyErrProxyExit         = "err.proxy-exit"
	KeyErrProxyHandshakeNeg = "err.proxy-handshake-neg"
	KeyErrProxyPublicListen = "err.proxy-public-listen"
	KeyErrProxySelfTarget   = "err.proxy-self-target"

	// proxy 内部处理错误（经日志展示给用户）。
	// SOCKS5 处理（socks5.go）。
	KeyErrProxySocksReadNMethods = "err.proxy-socks-read-nmethods"
	KeyErrProxySocksReadMethods  = "err.proxy-socks-read-methods"
	KeyErrProxySocksNoAuth       = "err.proxy-socks-no-auth"
	KeyErrProxySocksReplyAuth    = "err.proxy-socks-reply-auth"
	KeyErrProxySocksReadHeader   = "err.proxy-socks-read-header"
	KeyErrProxySocksBadVersion   = "err.proxy-socks-bad-version"
	KeyErrProxySocksParseAddr    = "err.proxy-socks-parse-addr"
	KeyErrProxySocksReadPort     = "err.proxy-socks-read-port"
	KeyErrProxySocksBadCommand   = "err.proxy-socks-bad-command"
	KeyErrProxySocksDial         = "err.proxy-socks-dial"
	KeyErrProxySocksReplySuccess = "err.proxy-socks-reply-success"
	KeyErrProxySocksBadAddrType  = "err.proxy-socks-bad-addr-type"

	// HTTP 处理（http.go）。
	KeyErrProxyHTTPParseRequest = "err.proxy-http-parse-request"
	KeyErrProxyHTTPConnectDial  = "err.proxy-http-connect-dial"
	KeyErrProxyHTTPConnectReply = "err.proxy-http-connect-reply"
	KeyErrProxyHTTPDial         = "err.proxy-http-dial"
	KeyErrProxyHTTPForward      = "err.proxy-http-forward"
	KeyErrProxyHTTPRelayResp    = "err.proxy-http-relay-resp"

	// proxy 上游代理链错误（upstream.go / main.go）。
	KeyErrProxyUpstreamScheme        = "err.proxy-upstream-scheme"
	KeyErrProxyUpstreamParse         = "err.proxy-upstream-parse"
	KeyErrProxyUpstreamEmptyHost     = "err.proxy-upstream-empty-host"
	KeyErrProxyUpstreamSocks5        = "err.proxy-upstream-socks5"
	KeyErrProxyUpstreamHTTPConnect   = "err.proxy-upstream-http-connect"
	KeyErrProxyUpstreamHTTPStatus    = "err.proxy-upstream-http-status"
	KeyErrProxyUpstreamSSHNoAuth     = "err.proxy-upstream-ssh-no-auth"
	KeyErrProxyUpstreamSSHIdentity   = "err.proxy-upstream-ssh-identity"
	KeyErrProxyUpstreamSSHParseKey   = "err.proxy-upstream-ssh-parse-key"
	KeyErrProxyUpstreamSSHKnownHosts = "err.proxy-upstream-ssh-known-hosts"
	KeyErrProxyUpstreamSSHDial       = "err.proxy-upstream-ssh-dial"
	KeyErrProxyUpstreamSSHChannel    = "err.proxy-upstream-ssh-channel"
	KeyErrProxyUpstreamClosed        = "err.proxy-upstream-closed"
)
