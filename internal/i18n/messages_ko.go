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

	KeyErrConfigRead:      "설정 파일 읽기에 실패했습니다: %w",
	KeyErrConfigParse:     "설정 파일 파싱에 실패했습니다: %w",
	KeyErrConfigDial:      "설정 파일의 dial_timeout이 잘못되었습니다: %w",
	KeyErrConfigIdle:      "설정 파일의 idle_timeout이 잘못되었습니다: %w",
	KeyErrConfigHandshake: "설정 파일의 handshake_timeout이 잘못되었습니다: %w",

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

	KeyUsageSubcommands: "하위 명령:\n  forward   TCP/UDP 포트 포워딩 (기본값)\n  proxy     단일 포트 SOCKS5 + HTTP 프록시\n  version   버전 정보 출력",
	KeyErrUnknownSub:    "알 수 없는 하위 명령: %q (forward, proxy 또는 version 선택)",

	KeyProxyUsageTitle: "portmap proxy - 단일 포트 SOCKS5 + HTTP 프록시",
	KeyProxyUsageLine:  "사용법: %s proxy [flags]\n\n단일 수신 포트에서 SOCKS5와 HTTP/HTTPS 클라이언트를 자동으로 감지합니다. 모든 아웃바운드 연결은 직접 다이얼하며 HTTP_PROXY/HTTPS_PROXY/ALL_PROXY를 무시합니다.\n\nflags:",

	KeyFlagProxyAddr:             "수신 주소, SOCKS5와 HTTP가 공유",
	KeyFlagProxyDialTimeout:      "아웃바운드 연결 타임아웃",
	KeyFlagProxyMaxConns:         "최대 동시 프록시 연결 수, 0은 무제한",
	KeyFlagProxyHandshakeTimeout: "프로토콜 핸드셰이크 타임아웃, 0은 비활성화",
	KeyFlagProxyIdleTimeout:      "양방향 유휴 타임아웃, 0은 비활성화",
	KeyFlagProxyAllowPublic:      "루프백이 아닌 주소 수신 허용(인증 기능 없음)",

	KeyLogProxyStarted:        "프록시 서비스가 시작되었습니다, %s에서 수신 중 (SOCKS5 + HTTP, 환경 프록시 무시)",
	KeyLogProxyAcceptFailed:   "연결 수락 실패: %v",
	KeyLogProxyDetectFailed:   "프로토콜 감지 실패 (%s): %v",
	KeyLogProxySOCKS5Failed:   "SOCKS5 처리 오류 (%s): %v",
	KeyLogProxyHTTPFailed:     "HTTP 처리 오류 (%s): %v",
	KeyLogProxySOCKS5Relay:    "SOCKS5 %s -> %s",
	KeyLogProxyHTTPConnect:    "HTTP CONNECT %s -> %s",
	KeyLogProxyHTTPPlain:      "HTTP %s %s -> %s",
	KeyLogProxyShuttingDown:   "종료 신호 수신, 종료 중...",
	KeyLogProxyShutdownFailed: "정상 종료가 완료되지 않았습니다: %v",
	KeyLogProxyConnLimit:      "%s 거부: 연결 한도 도달(%d)",

	KeyErrProxyExit:         "프록시 서비스가 비정상 종료되었습니다: %w",
	KeyErrProxyHandshakeNeg: "handshake-timeout은 음수일 수 없습니다: %s",
	KeyErrProxyPublicListen: "공개 주소 %s에서 인증 없는 프록시를 시작하지 않습니다. 허용하려면 -allow-public을 사용하세요",
	KeyErrProxySelfTarget:   "프록시 대상 %s이 현재 리스너로 확인되어 거부했습니다",

	KeyErrProxySocksReadNMethods: "NMETHODS 읽기 실패: %w",
	KeyErrProxySocksReadMethods:  "METHODS 읽기 실패: %w",
	KeyErrProxySocksNoAuth:       "클라이언트가 무인증 방식을 지원하지 않습니다",
	KeyErrProxySocksReplyAuth:    "인증 방식 응답 실패: %w",
	KeyErrProxySocksReadHeader:   "요청 헤더 읽기 실패: %w",
	KeyErrProxySocksBadVersion:   "잘못된 SOCKS 버전: %d",
	KeyErrProxySocksParseAddr:    "대상 주소 파싱 실패: %w",
	KeyErrProxySocksReadPort:     "포트 읽기 실패: %w",
	KeyErrProxySocksBadCommand:   "지원되지 않는 명령: %d",
	KeyErrProxySocksDial:         "대상 %s 연결 실패: %w",
	KeyErrProxySocksReplySuccess: "성공 응답 실패: %w",
	KeyErrProxySocksBadAddrType:  "지원되지 않는 주소 유형: %d",

	KeyErrProxyHTTPParseRequest: "HTTP 요청 파싱 실패: %w",
	KeyErrProxyHTTPConnectDial:  "%s 에 대한 CONNECT 연결 실패: %w",
	KeyErrProxyHTTPConnectReply: "CONNECT 성공 응답 실패: %w",
	KeyErrProxyHTTPDial:         "%s 연결 실패: %w",
	KeyErrProxyHTTPForward:      "%s 로 요청 전달 실패: %w",
	KeyErrProxyHTTPRelayResp:    "응답 회신 실패: %w",
}
