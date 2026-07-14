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

var messagesEN = map[string]string{
	KeyUsageTitle:  "portmap - TCP/UDP port forwarding (socat-equivalent)",
	KeyUsageLine:   "Usage: %s [flags]\n\nEquivalent to: sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222\n\nflags:",
	KeyVersionLine: "portmap %s (commit %s, built %s)",

	KeyFlagListenPort:  "local listen port",
	KeyFlagListenHost:  "local listen address (default: all interfaces)",
	KeyFlagTarget:      "forward target address host:port",
	KeyFlagMode:        "forward mode: go (pure Go) or socat (invoke system socat)",
	KeyFlagProto:       "forward protocol: tcp or udp",
	KeyFlagReuseAddr:   "enable SO_REUSEADDR",
	KeyFlagSudo:        "run via sudo in socat mode",
	KeyFlagDialTimeout: "dial timeout to target",
	KeyFlagMaxConns:    "max concurrent connections, 0 means unlimited (go mode only; limits concurrent sessions for UDP)",
	KeyFlagIdleTimeout: "idle timeout, disconnect when idle in both directions, 0 disables (go mode only; for UDP, 0 means default 60s session reclaim)",
	KeyFlagLogLevel:    "log level: info or debug (go mode only)",
	KeyFlagQuiet:       "quiet mode, suppress per-connection routine logs (go mode only)",
	KeyFlagVersion:     "print version info and exit",
	KeyFlagConfig:      "path to YAML config file",
	KeyFlagLang:        "interface language (%s); auto-detected from the system by default",

	KeyErrListenPort:  "invalid listen port: %d",
	KeyErrTargetEmpty: "target must not be empty",
	KeyErrProto:       "unknown proto: %q (choose tcp or udp)",
	KeyErrIdleNeg:     "idle-timeout must not be negative: %s",
	KeyErrMaxConnsNeg: "max-conns must not be negative: %d",
	KeyErrDialNeg:     "dial-timeout must not be negative: %s",
	KeyErrLogLevel:    "unknown log-level: %q (choose info or debug)",
	KeyErrMode:        "unknown mode: %q (choose go or socat)",
	KeyErrServeExit:   "forward service exited: %w",
	KeyErrSocatFailed: "socat execution failed: %w",

	KeyLogEffectiveConfig: "effective config: %s",
	KeyLogSocatIgnore:     "note: socat mode ignores the following go-mode-only parameters: %s",
	KeyLogSocatExec:       "executing: %s",
	KeyLogStatus:          "status: active=%d total=%d",

	KeyErrConfigRead:  "failed to read config file: %w",
	KeyErrConfigParse: "failed to parse config file: %w",
	KeyErrConfigDial:  "invalid dial_timeout in config file: %w",
	KeyErrConfigIdle:  "invalid idle_timeout in config file: %w",

	KeyErrUnsupportedNet:  "unsupported network: %q",
	KeyLogTCPListening:    "listening on %s (tcp), forwarding to %s (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogDialFailed:      "dial %s failed: %v",
	KeyLogConnOpen:        "[#%d] open %s <-> %s (active=%d)",
	KeyLogConnClose:       "[#%d] close %s <-> %s (up=%dB down=%dB dur=%s)",
	KeyLogPipeError:       "[#%d] pipe %s error: %v",
	KeyLogUDPListening:    "listening on %s (udp), forwarding to %s (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogUDPLimit:        "udp session limit reached, drop packet from %s",
	KeyLogUDPDialFailed:   "udp dial %s failed: %v",
	KeyLogUDPWriteTarget:  "udp write to target failed: %v",
	KeyLogUDPSessionOpen:  "[#%d] udp session %s <-> %s (active=%d)",
	KeyLogUDPSessionClose: "[#%d] udp session closed %s",
	KeyLogUDPWriteClient:  "udp write to client failed: %v",

	KeyErrSocatProto:      "invalid proto: %q",
	KeyErrSocatPort:       "invalid listen port: %d",
	KeyErrSocatTarget:     "empty target",
	KeyErrSocatNotFound:   "%q not found in PATH: %w",
	KeyErrSocatInvalidStr: "<invalid socat options>",
}
