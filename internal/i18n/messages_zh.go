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

	KeyErrConfigRead:  "读取配置文件失败: %w",
	KeyErrConfigParse: "解析配置文件失败: %w",
	KeyErrConfigDial:  "配置文件 dial_timeout 非法: %w",
	KeyErrConfigIdle:  "配置文件 idle_timeout 非法: %w",

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
}
