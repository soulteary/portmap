# Go Report Card

**Grade: A+** (97.3%)

| Metric | Value |
| ------ | ----- |
| Files | 48 |
| Issues | 14 |

## Checks

| Check | Score |
| ----- | ----- |
| go_vet | 100% |
| gofmt | 100% |
| ineffassign | 100% |
| gocyclo | 77% |
| license | 100% |
| misspell | 93% |

## Issues

### gocyclo

- `internal/proxy/http.go`
  - Line 98: cyclomatic complexity 17 for function (*Server).handlePlainHTTP
- `internal/forward/udp.go`
  - Line 44: cyclomatic complexity 16 for function (*Server).serveUDP
- `main.go`
  - Line 980: cyclomatic complexity 28 for function runProxyMulti
  - Line 145: cyclomatic complexity 26 for function runForward
  - Line 670: cyclomatic complexity 16 for function runProxy
- `config_test.go`
  - Line 437: cyclomatic complexity 23 for function TestMergeProxyConfigAllFields
  - Line 120: cyclomatic complexity 23 for function TestMergeConfig
  - Line 34: cyclomatic complexity 23 for function TestLoadConfig
- `internal/proxy/proxy_test.go`
  - Line 105: cyclomatic complexity 19 for function socks5Dial
  - Line 290: cyclomatic complexity 17 for function TestHTTPProxySanitizesBothDirectionsAndAddsVia
  - Line 359: cyclomatic complexity 16 for function TestHTTPProxyForwardsInformationalResponses
- `internal/proxy/server.go`
  - Line 131: cyclomatic complexity 18 for function (*Server).ListenAndServe
- `internal/proxy/events_test.go`
  - Line 106: cyclomatic complexity 17 for function TestProxyRecordsOpenCloseEvents
- `internal/proxy/socks5.go`
  - Line 62: cyclomatic complexity 16 for function (*Server).handleSOCKS5WithReader
- `config.go`
  - Line 338: cyclomatic complexity 41 for function applyProxyConfig
  - Line 444: cyclomatic complexity 36 for function mergeConfig
  - Line 512: cyclomatic complexity 34 for function applyForwardConfig
- `cmd/loadtest/main.go`
  - Line 64: cyclomatic complexity 25 for function parseFlags
  - Line 636: cyclomatic complexity 16 for function (*worker).runTCP
- `internal/forward/events_test.go`
  - Line 29: cyclomatic complexity 21 for function TestForwardRecordsOpenCloseEvents

### misspell

- `internal/i18n/messages_fr.go`
  - Line 37: "terminaison" is a misspelling of "termination"
  - Line 57: "terminaison" is a misspelling of "termination"
  - Line 58: "terminaison" is a misspelling of "termination"
  - Line 59: "terminaison" is a misspelling of "termination"
  - Line 60: "terminaison" is a misspelling of "termination"
  - Line 100: "marrage" is a misspelling of "marriage"
  - Line 101: "marrage" is a misspelling of "marriage"
  - Line 102: "marrage" is a misspelling of "marriage"
  - Line 105: "conflit" is a misspelling of "conflict"
  - Line 129: "commandes" is a misspelling of "commands"
- `.github/goreportcard-report.md`
  - Line 59: "Konfiguration" is a misspelling of "Configuration"
  - Line 60: "Konfiguration" is a misspelling of "Configuration"
  - Line 61: "Konfiguration" is a misspelling of "Configuration"
  - Line 62: "Konfiguration" is a misspelling of "Configuration"
  - Line 63: "Konfiguration" is a misspelling of "Configuration"
  - Line 64: "marrage" is a misspelling of "marriage"
  - Line 65: "marrage" is a misspelling of "marriage"
  - Line 66: "marrage" is a misspelling of "marriage"
  - Line 67: "commandes" is a misspelling of "commands"
  - Line 68: "Konfiguration" is a misspelling of "Configuration"
  - Line 69: "Konfiguration" is a misspelling of "Configuration"
  - Line 70: "commandes" is a misspelling of "commands"
  - Line 71: "Konfiguration" is a misspelling of "Configuration"
  - Line 72: "commandes" is a misspelling of "commands"
  - Line 73: "Konfiguration" is a misspelling of "Configuration"
  - Line 74: "commandes" is a misspelling of "commands"
  - Line 75: "Konfiguration" is a misspelling of "Configuration"
  - Line 76: "commandes" is a misspelling of "commands"
  - Line 77: "commandes" is a misspelling of "commands"
  - Line 78: "Konfiguration" is a misspelling of "Configuration"
  - Line 79: "terminaison" is a misspelling of "termination"
  - Line 80: "terminaison" is a misspelling of "termination"
  - Line 81: "terminaison" is a misspelling of "termination"
  - Line 82: "terminaison" is a misspelling of "termination"
  - Line 83: "terminaison" is a misspelling of "termination"
  - Line 84: "marrage" is a misspelling of "marriage"
  - Line 85: "marrage" is a misspelling of "marriage"
  - Line 86: "marrage" is a misspelling of "marriage"
  - Line 87: "commandes" is a misspelling of "commands"
  - Line 88: "Konfiguration" is a misspelling of "Configuration"
  - Line 89: "Konfiguration" is a misspelling of "Configuration"
  - Line 90: "Konfiguration" is a misspelling of "Configuration"
  - Line 91: "Konfiguration" is a misspelling of "Configuration"
  - Line 92: "Konfiguration" is a misspelling of "Configuration"
  - Line 93: "interaktive" is a misspelling of "interactive"
  - Line 94: "Konfiguration" is a misspelling of "Configuration"
  - Line 95: "Konfiguration" is a misspelling of "Configuration"
  - Line 96: "Konfiguration" is a misspelling of "Configuration"
  - Line 97: "Konfiguration" is a misspelling of "Configuration"
  - Line 98: "Konfiguration" is a misspelling of "Configuration"
  - Line 99: "interaktive" is a misspelling of "interactive"
  - Line 100: "terminaison" is a misspelling of "termination"
  - Line 101: "terminaison" is a misspelling of "termination"
  - Line 102: "terminaison" is a misspelling of "termination"
  - Line 103: "terminaison" is a misspelling of "termination"
  - Line 104: "terminaison" is a misspelling of "termination"
  - Line 105: "marrage" is a misspelling of "marriage"
  - Line 106: "marrage" is a misspelling of "marriage"
  - Line 107: "marrage" is a misspelling of "marriage"
  - Line 108: "commandes" is a misspelling of "commands"
  - Line 109: "Konfiguration" is a misspelling of "Configuration"
  - Line 110: "Konfiguration" is a misspelling of "Configuration"
  - Line 111: "Konfiguration" is a misspelling of "Configuration"
  - Line 112: "Konfiguration" is a misspelling of "Configuration"
  - Line 113: "Konfiguration" is a misspelling of "Configuration"
  - Line 114: "interaktive" is a misspelling of "interactive"
  - Line 115: "terminaison" is a misspelling of "termination"
  - Line 116: "terminaison" is a misspelling of "termination"
  - Line 117: "terminaison" is a misspelling of "termination"
  - Line 118: "terminaison" is a misspelling of "termination"
  - Line 119: "terminaison" is a misspelling of "termination"
  - Line 120: "marrage" is a misspelling of "marriage"
  - Line 121: "marrage" is a misspelling of "marriage"
  - Line 122: "marrage" is a misspelling of "marriage"
  - Line 123: "commandes" is a misspelling of "commands"
  - Line 125: "Konfiguration" is a misspelling of "Configuration"
  - Line 126: "Konfiguration" is a misspelling of "Configuration"
  - Line 127: "Konfiguration" is a misspelling of "Configuration"
  - Line 128: "Konfiguration" is a misspelling of "Configuration"
  - Line 129: "Konfiguration" is a misspelling of "Configuration"
  - Line 130: "interaktive" is a misspelling of "interactive"
  - Line 132: "terminaison" is a misspelling of "termination"
  - Line 133: "terminaison" is a misspelling of "termination"
  - Line 134: "terminaison" is a misspelling of "termination"
  - Line 135: "terminaison" is a misspelling of "termination"
  - Line 136: "terminaison" is a misspelling of "termination"
  - Line 137: "marrage" is a misspelling of "marriage"
  - Line 138: "marrage" is a misspelling of "marriage"
  - Line 139: "marrage" is a misspelling of "marriage"
  - Line 140: "conflit" is a misspelling of "conflict"
  - Line 141: "commandes" is a misspelling of "commands"
- `internal/i18n/messages_de.go`
  - Line 51: "Konfiguration" is a misspelling of "Configuration"
  - Line 100: "Konfiguration" is a misspelling of "Configuration"
  - Line 101: "Konfiguration" is a misspelling of "Configuration"
  - Line 104: "Konfiguration" is a misspelling of "Configuration"
  - Line 107: "Konfiguration" is a misspelling of "Configuration"
  - Line 204: "interaktive" is a misspelling of "interactive"

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action) on 2026-09-05 05:39:03 UTC._
