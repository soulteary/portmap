# Go Report Card

**Grade: A+** (97.5%)

| Metric | Value |
| ------ | ----- |
| Files | 48 |
| Issues | 13 |

## Checks

| Check | Score |
| ----- | ----- |
| go_vet | 100% |
| gofmt | 100% |
| ineffassign | 100% |
| gocyclo | 79% |
| license | 100% |
| misspell | 93% |

## Issues

### gocyclo

- `config_test.go`
  - Line 437: cyclomatic complexity 23 for function TestMergeProxyConfigAllFields
  - Line 120: cyclomatic complexity 23 for function TestMergeConfig
  - Line 34: cyclomatic complexity 23 for function TestLoadConfig
- `cmd/loadtest/main.go`
  - Line 477: cyclomatic complexity 16 for function (*worker).runTCP
- `config.go`
  - Line 336: cyclomatic complexity 37 for function applyProxyConfig
  - Line 436: cyclomatic complexity 36 for function mergeConfig
  - Line 504: cyclomatic complexity 34 for function applyForwardConfig
- `main.go`
  - Line 144: cyclomatic complexity 26 for function runForward
  - Line 912: cyclomatic complexity 23 for function runProxyMulti
  - Line 604: cyclomatic complexity 16 for function runProxy
- `internal/forward/events_test.go`
  - Line 29: cyclomatic complexity 21 for function TestForwardRecordsOpenCloseEvents
- `internal/proxy/proxy_test.go`
  - Line 105: cyclomatic complexity 19 for function socks5Dial
  - Line 290: cyclomatic complexity 17 for function TestHTTPProxySanitizesBothDirectionsAndAddsVia
  - Line 359: cyclomatic complexity 16 for function TestHTTPProxyForwardsInformationalResponses
- `internal/proxy/server.go`
  - Line 131: cyclomatic complexity 18 for function (*Server).ListenAndServe
- `internal/proxy/http.go`
  - Line 98: cyclomatic complexity 17 for function (*Server).handlePlainHTTP
- `internal/proxy/socks5.go`
  - Line 62: cyclomatic complexity 16 for function (*Server).handleSOCKS5WithReader
- `internal/forward/udp.go`
  - Line 44: cyclomatic complexity 16 for function (*Server).serveUDP

### misspell

- `.github/goreportcard-report.md`
  - Line 57: "Konfiguration" is a misspelling of "Configuration"
  - Line 58: "Konfiguration" is a misspelling of "Configuration"
  - Line 59: "Konfiguration" is a misspelling of "Configuration"
  - Line 60: "Konfiguration" is a misspelling of "Configuration"
  - Line 61: "Konfiguration" is a misspelling of "Configuration"
  - Line 62: "marrage" is a misspelling of "marriage"
  - Line 63: "marrage" is a misspelling of "marriage"
  - Line 64: "marrage" is a misspelling of "marriage"
  - Line 65: "commandes" is a misspelling of "commands"
  - Line 66: "Konfiguration" is a misspelling of "Configuration"
  - Line 67: "Konfiguration" is a misspelling of "Configuration"
  - Line 68: "commandes" is a misspelling of "commands"
  - Line 69: "Konfiguration" is a misspelling of "Configuration"
  - Line 70: "commandes" is a misspelling of "commands"
  - Line 71: "Konfiguration" is a misspelling of "Configuration"
  - Line 72: "commandes" is a misspelling of "commands"
  - Line 73: "Konfiguration" is a misspelling of "Configuration"
  - Line 74: "commandes" is a misspelling of "commands"
  - Line 75: "commandes" is a misspelling of "commands"
  - Line 76: "Konfiguration" is a misspelling of "Configuration"
  - Line 77: "terminaison" is a misspelling of "termination"
  - Line 78: "terminaison" is a misspelling of "termination"
  - Line 79: "terminaison" is a misspelling of "termination"
  - Line 80: "terminaison" is a misspelling of "termination"
  - Line 81: "terminaison" is a misspelling of "termination"
  - Line 82: "marrage" is a misspelling of "marriage"
  - Line 83: "marrage" is a misspelling of "marriage"
  - Line 84: "marrage" is a misspelling of "marriage"
  - Line 85: "commandes" is a misspelling of "commands"
  - Line 86: "Konfiguration" is a misspelling of "Configuration"
  - Line 87: "Konfiguration" is a misspelling of "Configuration"
  - Line 88: "Konfiguration" is a misspelling of "Configuration"
  - Line 89: "Konfiguration" is a misspelling of "Configuration"
  - Line 90: "Konfiguration" is a misspelling of "Configuration"
  - Line 91: "interaktive" is a misspelling of "interactive"
  - Line 93: "Konfiguration" is a misspelling of "Configuration"
  - Line 94: "Konfiguration" is a misspelling of "Configuration"
  - Line 95: "Konfiguration" is a misspelling of "Configuration"
  - Line 96: "Konfiguration" is a misspelling of "Configuration"
  - Line 97: "Konfiguration" is a misspelling of "Configuration"
  - Line 98: "interaktive" is a misspelling of "interactive"
  - Line 100: "terminaison" is a misspelling of "termination"
  - Line 101: "terminaison" is a misspelling of "termination"
  - Line 102: "terminaison" is a misspelling of "termination"
  - Line 103: "terminaison" is a misspelling of "termination"
  - Line 104: "terminaison" is a misspelling of "termination"
  - Line 105: "marrage" is a misspelling of "marriage"
  - Line 106: "marrage" is a misspelling of "marriage"
  - Line 107: "marrage" is a misspelling of "marriage"
  - Line 108: "commandes" is a misspelling of "commands"
- `internal/i18n/messages_de.go`
  - Line 50: "Konfiguration" is a misspelling of "Configuration"
  - Line 98: "Konfiguration" is a misspelling of "Configuration"
  - Line 99: "Konfiguration" is a misspelling of "Configuration"
  - Line 102: "Konfiguration" is a misspelling of "Configuration"
  - Line 103: "Konfiguration" is a misspelling of "Configuration"
  - Line 200: "interaktive" is a misspelling of "interactive"
- `internal/i18n/messages_fr.go`
  - Line 37: "terminaison" is a misspelling of "termination"
  - Line 56: "terminaison" is a misspelling of "termination"
  - Line 57: "terminaison" is a misspelling of "termination"
  - Line 58: "terminaison" is a misspelling of "termination"
  - Line 59: "terminaison" is a misspelling of "termination"
  - Line 98: "marrage" is a misspelling of "marriage"
  - Line 99: "marrage" is a misspelling of "marriage"
  - Line 100: "marrage" is a misspelling of "marriage"
  - Line 125: "commandes" is a misspelling of "commands"

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action) on 2026-09-04 17:54:02 UTC._
