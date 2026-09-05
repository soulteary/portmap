# Go Report Card

**Grade: A+** (97.4%)

| Metric | Value |
| ------ | ----- |
| Files | 50 |
| Issues | 14 |

## Checks

| Check | Score |
| ----- | ----- |
| go_vet | 100% |
| gofmt | 100% |
| ineffassign | 100% |
| gocyclo | 78% |
| license | 100% |
| misspell | 94% |

## Issues

### gocyclo

- `config.go`
  - Line 338: cyclomatic complexity 41 for function applyProxyConfig
  - Line 444: cyclomatic complexity 36 for function mergeConfig
  - Line 512: cyclomatic complexity 34 for function applyForwardConfig
- `main.go`
  - Line 980: cyclomatic complexity 28 for function runProxyMulti
  - Line 145: cyclomatic complexity 26 for function runForward
  - Line 670: cyclomatic complexity 16 for function runProxy
- `config_test.go`
  - Line 437: cyclomatic complexity 23 for function TestMergeProxyConfigAllFields
  - Line 120: cyclomatic complexity 23 for function TestMergeConfig
  - Line 34: cyclomatic complexity 23 for function TestLoadConfig
- `internal/proxy/socks5.go`
  - Line 62: cyclomatic complexity 18 for function (*Server).handleSOCKS5WithReader
- `internal/proxy/server.go`
  - Line 131: cyclomatic complexity 18 for function (*Server).ListenAndServe
- `internal/proxy/http.go`
  - Line 154: cyclomatic complexity 17 for function (*Server).handlePlainHTTP
- `cmd/loadtest/main.go`
  - Line 64: cyclomatic complexity 25 for function parseFlags
  - Line 636: cyclomatic complexity 16 for function (*worker).runTCP
- `internal/forward/events_test.go`
  - Line 29: cyclomatic complexity 21 for function TestForwardRecordsOpenCloseEvents
- `internal/proxy/proxy_test.go`
  - Line 105: cyclomatic complexity 19 for function socks5Dial
  - Line 290: cyclomatic complexity 17 for function TestHTTPProxySanitizesBothDirectionsAndAddsVia
  - Line 359: cyclomatic complexity 16 for function TestHTTPProxyForwardsInformationalResponses
- `internal/proxy/events_test.go`
  - Line 106: cyclomatic complexity 17 for function TestProxyRecordsOpenCloseEvents
- `internal/forward/udp.go`
  - Line 45: cyclomatic complexity 17 for function (*Server).serveUDP

### misspell

- `internal/i18n/messages_de.go`
  - Line 51: "Konfiguration" is a misspelling of "Configuration"
  - Line 100: "Konfiguration" is a misspelling of "Configuration"
  - Line 101: "Konfiguration" is a misspelling of "Configuration"
  - Line 104: "Konfiguration" is a misspelling of "Configuration"
  - Line 107: "Konfiguration" is a misspelling of "Configuration"
  - Line 205: "interaktive" is a misspelling of "interactive"
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
  - Line 130: "commandes" is a misspelling of "commands"
- `.github/goreportcard-report.md`
  - Line 60: "Konfiguration" is a misspelling of "Configuration"
  - Line 61: "Konfiguration" is a misspelling of "Configuration"
  - Line 62: "Konfiguration" is a misspelling of "Configuration"
  - Line 63: "Konfiguration" is a misspelling of "Configuration"
  - Line 64: "Konfiguration" is a misspelling of "Configuration"
  - Line 65: "interaktive" is a misspelling of "interactive"
  - Line 67: "terminaison" is a misspelling of "termination"
  - Line 68: "terminaison" is a misspelling of "termination"
  - Line 69: "terminaison" is a misspelling of "termination"
  - Line 70: "terminaison" is a misspelling of "termination"
  - Line 71: "terminaison" is a misspelling of "termination"
  - Line 72: "marrage" is a misspelling of "marriage"
  - Line 73: "marrage" is a misspelling of "marriage"
  - Line 74: "marrage" is a misspelling of "marriage"
  - Line 75: "conflit" is a misspelling of "conflict"
  - Line 76: "commandes" is a misspelling of "commands"
  - Line 78: "terminaison" is a misspelling of "termination"
  - Line 79: "terminaison" is a misspelling of "termination"
  - Line 80: "terminaison" is a misspelling of "termination"
  - Line 81: "terminaison" is a misspelling of "termination"
  - Line 82: "terminaison" is a misspelling of "termination"
  - Line 83: "marrage" is a misspelling of "marriage"
  - Line 84: "marrage" is a misspelling of "marriage"
  - Line 85: "marrage" is a misspelling of "marriage"
  - Line 86: "conflit" is a misspelling of "conflict"
  - Line 87: "commandes" is a misspelling of "commands"
  - Line 88: "Konfiguration" is a misspelling of "Configuration"
  - Line 89: "Konfiguration" is a misspelling of "Configuration"
  - Line 90: "Konfiguration" is a misspelling of "Configuration"
  - Line 91: "Konfiguration" is a misspelling of "Configuration"
  - Line 92: "Konfiguration" is a misspelling of "Configuration"
  - Line 93: "marrage" is a misspelling of "marriage"
  - Line 94: "marrage" is a misspelling of "marriage"
  - Line 95: "marrage" is a misspelling of "marriage"
  - Line 96: "commandes" is a misspelling of "commands"
  - Line 97: "Konfiguration" is a misspelling of "Configuration"
  - Line 98: "Konfiguration" is a misspelling of "Configuration"
  - Line 99: "commandes" is a misspelling of "commands"
  - Line 100: "Konfiguration" is a misspelling of "Configuration"
  - Line 101: "commandes" is a misspelling of "commands"
  - Line 102: "Konfiguration" is a misspelling of "Configuration"
  - Line 103: "commandes" is a misspelling of "commands"
  - Line 104: "Konfiguration" is a misspelling of "Configuration"
  - Line 105: "commandes" is a misspelling of "commands"
  - Line 106: "commandes" is a misspelling of "commands"
  - Line 107: "Konfiguration" is a misspelling of "Configuration"
  - Line 108: "terminaison" is a misspelling of "termination"
  - Line 109: "terminaison" is a misspelling of "termination"
  - Line 110: "terminaison" is a misspelling of "termination"
  - Line 111: "terminaison" is a misspelling of "termination"
  - Line 112: "terminaison" is a misspelling of "termination"
  - Line 113: "marrage" is a misspelling of "marriage"
  - Line 114: "marrage" is a misspelling of "marriage"
  - Line 115: "marrage" is a misspelling of "marriage"
  - Line 116: "commandes" is a misspelling of "commands"
  - Line 117: "Konfiguration" is a misspelling of "Configuration"
  - Line 118: "Konfiguration" is a misspelling of "Configuration"
  - Line 119: "Konfiguration" is a misspelling of "Configuration"
  - Line 120: "Konfiguration" is a misspelling of "Configuration"
  - Line 121: "Konfiguration" is a misspelling of "Configuration"
  - Line 122: "interaktive" is a misspelling of "interactive"
  - Line 123: "Konfiguration" is a misspelling of "Configuration"
  - Line 124: "Konfiguration" is a misspelling of "Configuration"
  - Line 125: "Konfiguration" is a misspelling of "Configuration"
  - Line 126: "Konfiguration" is a misspelling of "Configuration"
  - Line 127: "Konfiguration" is a misspelling of "Configuration"
  - Line 128: "interaktive" is a misspelling of "interactive"
  - Line 129: "terminaison" is a misspelling of "termination"
  - Line 130: "terminaison" is a misspelling of "termination"
  - Line 131: "terminaison" is a misspelling of "termination"
  - Line 132: "terminaison" is a misspelling of "termination"
  - Line 133: "terminaison" is a misspelling of "termination"
  - Line 134: "marrage" is a misspelling of "marriage"
  - Line 135: "marrage" is a misspelling of "marriage"
  - Line 136: "marrage" is a misspelling of "marriage"
  - Line 137: "commandes" is a misspelling of "commands"
  - Line 138: "Konfiguration" is a misspelling of "Configuration"
  - Line 139: "Konfiguration" is a misspelling of "Configuration"
  - Line 140: "Konfiguration" is a misspelling of "Configuration"
  - Line 141: "Konfiguration" is a misspelling of "Configuration"
  - Line 142: "Konfiguration" is a misspelling of "Configuration"
  - Line 143: "interaktive" is a misspelling of "interactive"
  - Line 144: "terminaison" is a misspelling of "termination"
  - Line 145: "terminaison" is a misspelling of "termination"
  - Line 146: "terminaison" is a misspelling of "termination"
  - Line 147: "terminaison" is a misspelling of "termination"
  - Line 148: "terminaison" is a misspelling of "termination"
  - Line 149: "marrage" is a misspelling of "marriage"
  - Line 150: "marrage" is a misspelling of "marriage"
  - Line 151: "marrage" is a misspelling of "marriage"
  - Line 152: "commandes" is a misspelling of "commands"
  - Line 153: "Konfiguration" is a misspelling of "Configuration"
  - Line 154: "Konfiguration" is a misspelling of "Configuration"
  - Line 155: "Konfiguration" is a misspelling of "Configuration"
  - Line 156: "Konfiguration" is a misspelling of "Configuration"
  - Line 157: "Konfiguration" is a misspelling of "Configuration"
  - Line 158: "interaktive" is a misspelling of "interactive"
  - Line 159: "terminaison" is a misspelling of "termination"
  - Line 160: "terminaison" is a misspelling of "termination"
  - Line 161: "terminaison" is a misspelling of "termination"
  - Line 162: "terminaison" is a misspelling of "termination"
  - Line 163: "terminaison" is a misspelling of "termination"
  - Line 164: "marrage" is a misspelling of "marriage"
  - Line 165: "marrage" is a misspelling of "marriage"
  - Line 166: "marrage" is a misspelling of "marriage"
  - Line 167: "conflit" is a misspelling of "conflict"
  - Line 168: "commandes" is a misspelling of "commands"
  - Line 169: "Konfiguration" is a misspelling of "Configuration"
  - Line 170: "Konfiguration" is a misspelling of "Configuration"
  - Line 171: "Konfiguration" is a misspelling of "Configuration"
  - Line 172: "Konfiguration" is a misspelling of "Configuration"
  - Line 173: "Konfiguration" is a misspelling of "Configuration"
  - Line 174: "interaktive" is a misspelling of "interactive"

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action) on 2026-09-05 05:59:27 UTC._
