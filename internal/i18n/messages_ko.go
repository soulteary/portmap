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

var messagesKO = map[string]string{
	KeyUsageTitle:  "portmap - TCP/UDP 포트 포워딩 (socat 동등 구현)",
	KeyUsageLine:   "사용법: %s [flags]\n\n다음과 동등: sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222\n\nflags:",
	KeyVersionLine: "portmap %s (commit %s, built %s)",

	KeyFlagListenPort:  "로컬 수신 포트",
	KeyFlagListenHost:  "로컬 수신 주소 (기본값: 모든 인터페이스)",
	KeyFlagTarget:      "전달 대상 주소 host:port",
	KeyFlagMode:        "전달 모드: go(순수 Go 구현) 또는 socat(시스템 socat 호출)",
	KeyFlagProto:       "전달 프로토콜: tcp 또는 udp",
	KeyFlagReuseAddr:   "SO_REUSEADDR 활성화",
	KeyFlagSudo:        "socat 모드에서 sudo로 실행할지 여부",
	KeyFlagDialTimeout: "대상으로의 다이얼 타임아웃",
	KeyFlagMaxConns:    "최대 동시 연결 수, 0은 무제한 (go 모드 전용; UDP에서는 동시 세션 수 제한)",
	KeyFlagIdleTimeout: "유휴 타임아웃, 양방향 무통신 시 연결 해제, 0은 비활성화 (go 모드 전용; UDP에서 0은 기본 60초 세션 회수)",
	KeyFlagLogLevel:    "로그 레벨: info 또는 debug (go 모드 전용)",
	KeyFlagQuiet:       "조용한 모드, 연결별 일반 로그 억제 (go 모드 전용)",
	KeyFlagVersion:     "버전 정보를 출력하고 종료",
	KeyFlagConfig:      "YAML 설정 파일 경로",
	KeyFlagLang:        "인터페이스 언어 (%s); 기본적으로 시스템에서 자동 감지",

	KeyErrListenPort:  "잘못된 수신 포트: %d",
	KeyErrTargetEmpty: "target은 비어 있을 수 없습니다",
	KeyErrProto:       "알 수 없는 proto: %q (tcp 또는 udp 선택)",
	KeyErrIdleNeg:     "idle-timeout은 음수일 수 없습니다: %s",
	KeyErrMaxConnsNeg: "max-conns는 음수일 수 없습니다: %d",
	KeyErrDialNeg:     "dial-timeout은 음수일 수 없습니다: %s",
	KeyErrLogLevel:    "알 수 없는 log-level: %q (info 또는 debug 선택)",
	KeyErrMode:        "알 수 없는 mode: %q (go 또는 socat 선택)",
	KeyErrServeExit:   "전달 서비스가 종료되었습니다: %w",
	KeyErrSocatFailed: "socat 실행에 실패했습니다: %w",

	KeyLogEffectiveConfig: "적용된 설정: %s",
	KeyLogSocatIgnore:     "참고: socat 모드는 다음 go 모드 전용 매개변수를 무시합니다: %s",
	KeyLogSocatExec:       "실행 중: %s",
	KeyLogStatus:          "status: active=%d total=%d",

	KeyErrConfigRead:  "설정 파일 읽기에 실패했습니다: %w",
	KeyErrConfigParse: "설정 파일 파싱에 실패했습니다: %w",
	KeyErrConfigDial:  "설정 파일의 dial_timeout이 잘못되었습니다: %w",
	KeyErrConfigIdle:  "설정 파일의 idle_timeout이 잘못되었습니다: %w",

	KeyErrUnsupportedNet:  "지원되지 않는 네트워크: %q",
	KeyLogTCPListening:    "%s (tcp)에서 수신 중, %s로 전달 (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogDialFailed:      "%s 다이얼 실패: %v",
	KeyLogConnOpen:        "[#%d] 연결 열림 %s <-> %s (active=%d)",
	KeyLogConnClose:       "[#%d] 연결 닫힘 %s <-> %s (up=%dB down=%dB dur=%s)",
	KeyLogPipeError:       "[#%d] 전달 %s 오류: %v",
	KeyLogUDPListening:    "%s (udp)에서 수신 중, %s로 전달 (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogUDPLimit:        "udp 세션 수가 한도에 도달, %s의 패킷을 폐기",
	KeyLogUDPDialFailed:   "udp 다이얼 %s 실패: %v",
	KeyLogUDPWriteTarget:  "udp 대상 쓰기 실패: %v",
	KeyLogUDPSessionOpen:  "[#%d] udp 세션 %s <-> %s (active=%d)",
	KeyLogUDPSessionClose: "[#%d] udp 세션 닫힘 %s",
	KeyLogUDPWriteClient:  "udp 클라이언트 쓰기 실패: %v",

	KeyErrSocatProto:      "잘못된 proto: %q",
	KeyErrSocatPort:       "잘못된 수신 포트: %d",
	KeyErrSocatTarget:     "target이 비어 있습니다",
	KeyErrSocatNotFound:   "PATH에서 %q를 찾을 수 없습니다: %w",
	KeyErrSocatInvalidStr: "<잘못된 socat 옵션>",
}
