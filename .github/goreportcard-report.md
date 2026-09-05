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

- `config.go`
  - Line 338: cyclomatic complexity 41 for function applyProxyConfig
  - Line 444: cyclomatic complexity 36 for function mergeConfig
  - Line 512: cyclomatic complexity 34 for function applyForwardConfig
- `internal/forward/events_test.go`
  - Line 29: cyclomatic complexity 21 for function TestForwardRecordsOpenCloseEvents
- `internal/proxy/proxy_test.go`
  - Line 105: cyclomatic complexity 19 for function socks5Dial
  - Line 290: cyclomatic complexity 17 for function TestHTTPProxySanitizesBothDirectionsAndAddsVia
  - Line 359: cyclomatic complexity 16 for function TestHTTPProxyForwardsInformationalResponses
- `internal/proxy/http.go`
  - Line 154: cyclomatic complexity 17 for function (*Server).handlePlainHTTP
- `internal/proxy/events_test.go`
  - Line 106: cyclomatic complexity 17 for function TestProxyRecordsOpenCloseEvents
- `internal/forward/udp.go`
  - Line 44: cyclomatic complexity 16 for function (*Server).serveUDP
- `main.go`
  - Line 980: cyclomatic complexity 28 for function runProxyMulti
  - Line 145: cyclomatic complexity 26 for function runForward
  - Line 670: cyclomatic complexity 16 for function runProxy
- `cmd/loadtest/main.go`
  - Line 64: cyclomatic complexity 25 for function parseFlags
  - Line 636: cyclomatic complexity 16 for function (*worker).runTCP
- `config_test.go`
  - Line 437: cyclomatic complexity 23 for function TestMergeProxyConfigAllFields
  - Line 120: cyclomatic complexity 23 for function TestMergeConfig
  - Line 34: cyclomatic complexity 23 for function TestLoadConfig
- `internal/proxy/socks5.go`
  - Line 62: cyclomatic complexity 18 for function (*Server).handleSOCKS5WithReader
- `internal/proxy/server.go`
  - Line 131: cyclomatic complexity 18 for function (*Server).ListenAndServe

### misspell

- `internal/i18n/messages_de.go`
  - Line 51: "Konfiguration" is a misspelling of "Configuration"
  - Line 100: "Konfiguration" is a misspelling of "Configuration"
  - Line 101: "Konfiguration" is a misspelling of "Configuration"
  - Line 104: "Konfiguration" is a misspelling of "Configuration"
  - Line 107: "Konfiguration" is a misspelling of "Configuration"
  - Line 204: "interaktive" is a misspelling of "interactive"
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
  - Line 60: "terminaison" is a misspelling of "termination"
  - Line 61: "terminaison" is a misspelling of "termination"
  - Line 62: "terminaison" is a misspelling of "termination"
  - Line 63: "terminaison" is a misspelling of "termination"
  - Line 64: "terminaison" is a misspelling of "termination"
  - Line 65: "marrage" is a misspelling of "marriage"
  - Line 66: "marrage" is a misspelling of "marriage"
  - Line 67: "marrage" is a misspelling of "marriage"
  - Line 68: "conflit" is a misspelling of "conflict"
  - Line 69: "commandes" is a misspelling of "commands"
  - Line 71: "Konfiguration" is a misspelling of "Configuration"
  - Line 72: "Konfiguration" is a misspelling of "Configuration"
  - Line 73: "Konfiguration" is a misspelling of "Configuration"
  - Line 74: "Konfiguration" is a misspelling of "Configuration"
  - Line 75: "Konfiguration" is a misspelling of "Configuration"
  - Line 76: "marrage" is a misspelling of "marriage"
  - Line 77: "marrage" is a misspelling of "marriage"
  - Line 78: "marrage" is a misspelling of "marriage"
  - Line 79: "commandes" is a misspelling of "commands"
  - Line 80: "Konfiguration" is a misspelling of "Configuration"
  - Line 81: "Konfiguration" is a misspelling of "Configuration"
  - Line 82: "commandes" is a misspelling of "commands"
  - Line 83: "Konfiguration" is a misspelling of "Configuration"
  - Line 84: "commandes" is a misspelling of "commands"
  - Line 85: "Konfiguration" is a misspelling of "Configuration"
  - Line 86: "commandes" is a misspelling of "commands"
  - Line 87: "Konfiguration" is a misspelling of "Configuration"
  - Line 88: "commandes" is a misspelling of "commands"
  - Line 89: "commandes" is a misspelling of "commands"
  - Line 90: "Konfiguration" is a misspelling of "Configuration"
  - Line 91: "terminaison" is a misspelling of "termination"
  - Line 92: "terminaison" is a misspelling of "termination"
  - Line 93: "terminaison" is a misspelling of "termination"
  - Line 94: "terminaison" is a misspelling of "termination"
  - Line 95: "terminaison" is a misspelling of "termination"
  - Line 96: "marrage" is a misspelling of "marriage"
  - Line 97: "marrage" is a misspelling of "marriage"
  - Line 98: "marrage" is a misspelling of "marriage"
  - Line 99: "commandes" is a misspelling of "commands"
  - Line 100: "Konfiguration" is a misspelling of "Configuration"
  - Line 101: "Konfiguration" is a misspelling of "Configuration"
  - Line 102: "Konfiguration" is a misspelling of "Configuration"
  - Line 103: "Konfiguration" is a misspelling of "Configuration"
  - Line 104: "Konfiguration" is a misspelling of "Configuration"
  - Line 105: "interaktive" is a misspelling of "interactive"
  - Line 106: "Konfiguration" is a misspelling of "Configuration"
  - Line 107: "Konfiguration" is a misspelling of "Configuration"
  - Line 108: "Konfiguration" is a misspelling of "Configuration"
  - Line 109: "Konfiguration" is a misspelling of "Configuration"
  - Line 110: "Konfiguration" is a misspelling of "Configuration"
  - Line 111: "interaktive" is a misspelling of "interactive"
  - Line 112: "terminaison" is a misspelling of "termination"
  - Line 113: "terminaison" is a misspelling of "termination"
  - Line 114: "terminaison" is a misspelling of "termination"
  - Line 115: "terminaison" is a misspelling of "termination"
  - Line 116: "terminaison" is a misspelling of "termination"
  - Line 117: "marrage" is a misspelling of "marriage"
  - Line 118: "marrage" is a misspelling of "marriage"
  - Line 119: "marrage" is a misspelling of "marriage"
  - Line 120: "commandes" is a misspelling of "commands"
  - Line 121: "Konfiguration" is a misspelling of "Configuration"
  - Line 122: "Konfiguration" is a misspelling of "Configuration"
  - Line 123: "Konfiguration" is a misspelling of "Configuration"
  - Line 124: "Konfiguration" is a misspelling of "Configuration"
  - Line 125: "Konfiguration" is a misspelling of "Configuration"
  - Line 126: "interaktive" is a misspelling of "interactive"
  - Line 127: "terminaison" is a misspelling of "termination"
  - Line 128: "terminaison" is a misspelling of "termination"
  - Line 129: "terminaison" is a misspelling of "termination"
  - Line 130: "terminaison" is a misspelling of "termination"
  - Line 131: "terminaison" is a misspelling of "termination"
  - Line 132: "marrage" is a misspelling of "marriage"
  - Line 133: "marrage" is a misspelling of "marriage"
  - Line 134: "marrage" is a misspelling of "marriage"
  - Line 135: "commandes" is a misspelling of "commands"
  - Line 136: "Konfiguration" is a misspelling of "Configuration"
  - Line 137: "Konfiguration" is a misspelling of "Configuration"
  - Line 138: "Konfiguration" is a misspelling of "Configuration"
  - Line 139: "Konfiguration" is a misspelling of "Configuration"
  - Line 140: "Konfiguration" is a misspelling of "Configuration"
  - Line 141: "interaktive" is a misspelling of "interactive"
  - Line 142: "terminaison" is a misspelling of "termination"
  - Line 143: "terminaison" is a misspelling of "termination"
  - Line 144: "terminaison" is a misspelling of "termination"
  - Line 145: "terminaison" is a misspelling of "termination"
  - Line 146: "terminaison" is a misspelling of "termination"
  - Line 147: "marrage" is a misspelling of "marriage"
  - Line 148: "marrage" is a misspelling of "marriage"
  - Line 149: "marrage" is a misspelling of "marriage"
  - Line 150: "conflit" is a misspelling of "conflict"
  - Line 151: "commandes" is a misspelling of "commands"
  - Line 153: "Konfiguration" is a misspelling of "Configuration"
  - Line 154: "Konfiguration" is a misspelling of "Configuration"
  - Line 155: "Konfiguration" is a misspelling of "Configuration"
  - Line 156: "Konfiguration" is a misspelling of "Configuration"
  - Line 157: "Konfiguration" is a misspelling of "Configuration"
  - Line 158: "interaktive" is a misspelling of "interactive"

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action) on 2026-09-05 05:42:21 UTC._
