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

var messagesJA = map[string]string{
	KeyUsageTitle:  "portmap - TCP/UDP ポートフォワーディング (socat 相当)",
	KeyUsageLine:   "使い方: %s [flags]\n\n次と同等: sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222\n\nflags:",
	KeyVersionLine: "portmap %s (commit %s, built %s)",

	KeyFlagListenPort:  "ローカル待ち受けポート",
	KeyFlagListenHost:  "ローカル待ち受けアドレス（既定: すべてのインターフェース）",
	KeyFlagTarget:      "転送先アドレス host:port",
	KeyFlagMode:        "転送モード: go（純 Go 実装）または socat（システムの socat を呼び出す）",
	KeyFlagProto:       "転送プロトコル: tcp または udp",
	KeyFlagReuseAddr:   "SO_REUSEADDR を有効化",
	KeyFlagSudo:        "socat モードで sudo を使って実行するか",
	KeyFlagDialTimeout: "転送先へのダイヤルタイムアウト",
	KeyFlagMaxConns:    "最大同時接続数、0 は無制限（go モードのみ; UDP では同時セッション数を制限）",
	KeyFlagIdleTimeout: "アイドルタイムアウト、双方向で無通信なら切断、0 は無効（go モードのみ; UDP では 0 は既定 60 秒でセッション回収）",
	KeyFlagLogLevel:    "ログレベル: info または debug（go モードのみ）",
	KeyFlagQuiet:       "サイレントモード、接続ごとの通常ログを抑制（go モードのみ）",
	KeyFlagVersion:     "バージョン情報を表示して終了",
	KeyFlagConfig:      "YAML 設定ファイルのパス",
	KeyFlagLang:        "インターフェース言語（%s）; 既定ではシステムから自動検出",

	KeyErrListenPort:  "不正な待ち受けポート: %d",
	KeyErrTargetEmpty: "target は空にできません",
	KeyErrProto:       "不明な proto: %q（tcp または udp を選択）",
	KeyErrIdleNeg:     "idle-timeout は負にできません: %s",
	KeyErrMaxConnsNeg: "max-conns は負にできません: %d",
	KeyErrDialNeg:     "dial-timeout は負にできません: %s",
	KeyErrLogLevel:    "不明な log-level: %q（info または debug を選択）",
	KeyErrMode:        "不明な mode: %q（go または socat を選択）",
	KeyErrServeExit:   "転送サービスが終了しました: %w",
	KeyErrSocatFailed: "socat の実行に失敗しました: %w",

	KeyLogEffectiveConfig: "有効な設定: %s",
	KeyLogSocatIgnore:     "注意: socat モードでは次の go モード専用パラメータを無視します: %s",
	KeyLogSocatExec:       "実行中: %s",
	KeyLogStatus:          "status: active=%d total=%d",

	KeyErrConfigRead:  "設定ファイルの読み込みに失敗しました: %w",
	KeyErrConfigParse: "設定ファイルの解析に失敗しました: %w",
	KeyErrConfigDial:  "設定ファイルの dial_timeout が不正です: %w",
	KeyErrConfigIdle:  "設定ファイルの idle_timeout が不正です: %w",

	KeyErrUnsupportedNet:  "サポートされていないネットワーク: %q",
	KeyLogTCPListening:    "%s (tcp) で待ち受け中、%s へ転送 (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogDialFailed:      "%s へのダイヤルに失敗: %v",
	KeyLogConnOpen:        "[#%d] 接続開始 %s <-> %s (active=%d)",
	KeyLogConnClose:       "[#%d] 接続終了 %s <-> %s (up=%dB down=%dB dur=%s)",
	KeyLogPipeError:       "[#%d] 転送 %s エラー: %v",
	KeyLogUDPListening:    "%s (udp) で待ち受け中、%s へ転送 (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogUDPLimit:        "udp セッション数が上限に達しました、%s からのパケットを破棄",
	KeyLogUDPDialFailed:   "udp ダイヤル %s に失敗: %v",
	KeyLogUDPWriteTarget:  "udp の転送先への書き込みに失敗: %v",
	KeyLogUDPSessionOpen:  "[#%d] udp セッション %s <-> %s (active=%d)",
	KeyLogUDPSessionClose: "[#%d] udp セッション終了 %s",
	KeyLogUDPWriteClient:  "udp のクライアントへの書き込みに失敗: %v",

	KeyErrSocatProto:      "不正な proto: %q",
	KeyErrSocatPort:       "不正な待ち受けポート: %d",
	KeyErrSocatTarget:     "target が空です",
	KeyErrSocatNotFound:   "%q が PATH に見つかりません: %w",
	KeyErrSocatInvalidStr: "<不正な socat オプション>",
}
