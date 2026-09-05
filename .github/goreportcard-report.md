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

- `internal/proxy/proxy_test.go`
  - Line 105: cyclomatic complexity 19 for function socks5Dial
  - Line 290: cyclomatic complexity 17 for function TestHTTPProxySanitizesBothDirectionsAndAddsVia
  - Line 359: cyclomatic complexity 16 for function TestHTTPProxyForwardsInformationalResponses
- `config.go`
  - Line 338: cyclomatic complexity 41 for function applyProxyConfig
  - Line 444: cyclomatic complexity 36 for function mergeConfig
  - Line 512: cyclomatic complexity 34 for function applyForwardConfig
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
- `internal/proxy/http.go`
  - Line 154: cyclomatic complexity 17 for function (*Server).handlePlainHTTP
- `internal/proxy/events_test.go`
  - Line 106: cyclomatic complexity 17 for function TestProxyRecordsOpenCloseEvents
- `internal/forward/udp.go`
  - Line 45: cyclomatic complexity 17 for function (*Server).serveUDP
- `main.go`
  - Line 980: cyclomatic complexity 28 for function runProxyMulti
  - Line 145: cyclomatic complexity 26 for function runForward
  - Line 670: cyclomatic complexity 16 for function runProxy
- `internal/forward/events_test.go`
  - Line 29: cyclomatic complexity 21 for function TestForwardRecordsOpenCloseEvents

### misspell

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
  - Line 78: "Konfiguration" is a misspelling of "Configuration"
  - Line 79: "Konfiguration" is a misspelling of "Configuration"
  - Line 80: "Konfiguration" is a misspelling of "Configuration"
  - Line 81: "Konfiguration" is a misspelling of "Configuration"
  - Line 82: "Konfiguration" is a misspelling of "Configuration"
  - Line 83: "interaktive" is a misspelling of "interactive"
  - Line 84: "terminaison" is a misspelling of "termination"
  - Line 85: "terminaison" is a misspelling of "termination"
  - Line 86: "terminaison" is a misspelling of "termination"
  - Line 87: "terminaison" is a misspelling of "termination"
  - Line 88: "terminaison" is a misspelling of "termination"
  - Line 89: "marrage" is a misspelling of "marriage"
  - Line 90: "marrage" is a misspelling of "marriage"
  - Line 91: "marrage" is a misspelling of "marriage"
  - Line 92: "conflit" is a misspelling of "conflict"
  - Line 93: "commandes" is a misspelling of "commands"
  - Line 94: "terminaison" is a misspelling of "termination"
  - Line 95: "terminaison" is a misspelling of "termination"
  - Line 96: "terminaison" is a misspelling of "termination"
  - Line 97: "terminaison" is a misspelling of "termination"
  - Line 98: "terminaison" is a misspelling of "termination"
  - Line 99: "marrage" is a misspelling of "marriage"
  - Line 100: "marrage" is a misspelling of "marriage"
  - Line 101: "marrage" is a misspelling of "marriage"
  - Line 102: "conflit" is a misspelling of "conflict"
  - Line 103: "commandes" is a misspelling of "commands"
  - Line 104: "Konfiguration" is a misspelling of "Configuration"
  - Line 105: "Konfiguration" is a misspelling of "Configuration"
  - Line 106: "Konfiguration" is a misspelling of "Configuration"
  - Line 107: "Konfiguration" is a misspelling of "Configuration"
  - Line 108: "Konfiguration" is a misspelling of "Configuration"
  - Line 109: "marrage" is a misspelling of "marriage"
  - Line 110: "marrage" is a misspelling of "marriage"
  - Line 111: "marrage" is a misspelling of "marriage"
  - Line 112: "commandes" is a misspelling of "commands"
  - Line 113: "Konfiguration" is a misspelling of "Configuration"
  - Line 114: "Konfiguration" is a misspelling of "Configuration"
  - Line 115: "commandes" is a misspelling of "commands"
  - Line 116: "Konfiguration" is a misspelling of "Configuration"
  - Line 117: "commandes" is a misspelling of "commands"
  - Line 118: "Konfiguration" is a misspelling of "Configuration"
  - Line 119: "commandes" is a misspelling of "commands"
  - Line 120: "Konfiguration" is a misspelling of "Configuration"
  - Line 121: "commandes" is a misspelling of "commands"
  - Line 122: "commandes" is a misspelling of "commands"
  - Line 123: "Konfiguration" is a misspelling of "Configuration"
  - Line 124: "terminaison" is a misspelling of "termination"
  - Line 125: "terminaison" is a misspelling of "termination"
  - Line 126: "terminaison" is a misspelling of "termination"
  - Line 127: "terminaison" is a misspelling of "termination"
  - Line 128: "terminaison" is a misspelling of "termination"
  - Line 129: "marrage" is a misspelling of "marriage"
  - Line 130: "marrage" is a misspelling of "marriage"
  - Line 131: "marrage" is a misspelling of "marriage"
  - Line 132: "commandes" is a misspelling of "commands"
  - Line 133: "Konfiguration" is a misspelling of "Configuration"
  - Line 134: "Konfiguration" is a misspelling of "Configuration"
  - Line 135: "Konfiguration" is a misspelling of "Configuration"
  - Line 136: "Konfiguration" is a misspelling of "Configuration"
  - Line 137: "Konfiguration" is a misspelling of "Configuration"
  - Line 138: "interaktive" is a misspelling of "interactive"
  - Line 139: "Konfiguration" is a misspelling of "Configuration"
  - Line 140: "Konfiguration" is a misspelling of "Configuration"
  - Line 141: "Konfiguration" is a misspelling of "Configuration"
  - Line 142: "Konfiguration" is a misspelling of "Configuration"
  - Line 143: "Konfiguration" is a misspelling of "Configuration"
  - Line 144: "interaktive" is a misspelling of "interactive"
  - Line 145: "terminaison" is a misspelling of "termination"
  - Line 146: "terminaison" is a misspelling of "termination"
  - Line 147: "terminaison" is a misspelling of "termination"
  - Line 148: "terminaison" is a misspelling of "termination"
  - Line 149: "terminaison" is a misspelling of "termination"
  - Line 150: "marrage" is a misspelling of "marriage"
  - Line 151: "marrage" is a misspelling of "marriage"
  - Line 152: "marrage" is a misspelling of "marriage"
  - Line 153: "commandes" is a misspelling of "commands"
  - Line 154: "Konfiguration" is a misspelling of "Configuration"
  - Line 155: "Konfiguration" is a misspelling of "Configuration"
  - Line 156: "Konfiguration" is a misspelling of "Configuration"
  - Line 157: "Konfiguration" is a misspelling of "Configuration"
  - Line 158: "Konfiguration" is a misspelling of "Configuration"
  - Line 159: "interaktive" is a misspelling of "interactive"
  - Line 160: "terminaison" is a misspelling of "termination"
  - Line 161: "terminaison" is a misspelling of "termination"
  - Line 162: "terminaison" is a misspelling of "termination"
  - Line 163: "terminaison" is a misspelling of "termination"
  - Line 164: "terminaison" is a misspelling of "termination"
  - Line 165: "marrage" is a misspelling of "marriage"
  - Line 166: "marrage" is a misspelling of "marriage"
  - Line 167: "marrage" is a misspelling of "marriage"
  - Line 168: "commandes" is a misspelling of "commands"
  - Line 169: "Konfiguration" is a misspelling of "Configuration"
  - Line 170: "Konfiguration" is a misspelling of "Configuration"
  - Line 171: "Konfiguration" is a misspelling of "Configuration"
  - Line 172: "Konfiguration" is a misspelling of "Configuration"
  - Line 173: "Konfiguration" is a misspelling of "Configuration"
  - Line 174: "interaktive" is a misspelling of "interactive"
  - Line 175: "terminaison" is a misspelling of "termination"
  - Line 176: "terminaison" is a misspelling of "termination"
  - Line 177: "terminaison" is a misspelling of "termination"
  - Line 178: "terminaison" is a misspelling of "termination"
  - Line 179: "terminaison" is a misspelling of "termination"
  - Line 180: "marrage" is a misspelling of "marriage"
  - Line 181: "marrage" is a misspelling of "marriage"
  - Line 182: "marrage" is a misspelling of "marriage"
  - Line 183: "conflit" is a misspelling of "conflict"
  - Line 184: "commandes" is a misspelling of "commands"
  - Line 185: "Konfiguration" is a misspelling of "Configuration"
  - Line 186: "Konfiguration" is a misspelling of "Configuration"
  - Line 187: "Konfiguration" is a misspelling of "Configuration"
  - Line 188: "Konfiguration" is a misspelling of "Configuration"
  - Line 189: "Konfiguration" is a misspelling of "Configuration"
  - Line 190: "interaktive" is a misspelling of "interactive"
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

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action) on 2026-09-05 06:22:51 UTC._
