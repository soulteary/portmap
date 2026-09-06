# Go Report Card

**Grade: A+** (97.2%)

| Metric | Value |
| ------ | ----- |
| Files | 50 |
| Issues | 15 |

## Checks

| Check | Score |
| ----- | ----- |
| go_vet | 100% |
| gofmt | 100% |
| ineffassign | 100% |
| gocyclo | 76% |
| license | 100% |
| misspell | 94% |

## Issues

### gocyclo

- `config.go`
  - Line 340: cyclomatic complexity 45 for function applyProxyConfig
  - Line 452: cyclomatic complexity 36 for function mergeConfig
  - Line 520: cyclomatic complexity 34 for function applyForwardConfig
- `config_test.go`
  - Line 437: cyclomatic complexity 28 for function TestMergeProxyConfigAllFields
  - Line 120: cyclomatic complexity 23 for function TestMergeConfig
  - Line 34: cyclomatic complexity 23 for function TestLoadConfig
- `cmd/loadtest/main.go`
  - Line 64: cyclomatic complexity 25 for function parseFlags
  - Line 636: cyclomatic complexity 16 for function (*worker).runTCP
- `coverage_test.go`
  - Line 158: cyclomatic complexity 24 for function TestBuildProxyUpstream
- `internal/forward/events_test.go`
  - Line 29: cyclomatic complexity 21 for function TestForwardRecordsOpenCloseEvents
- `internal/proxy/proxy_test.go`
  - Line 105: cyclomatic complexity 19 for function socks5Dial
  - Line 290: cyclomatic complexity 17 for function TestHTTPProxySanitizesBothDirectionsAndAddsVia
  - Line 359: cyclomatic complexity 16 for function TestHTTPProxyForwardsInformationalResponses
- `internal/proxy/socks5.go`
  - Line 62: cyclomatic complexity 18 for function (*Server).handleSOCKS5WithReader
- `internal/proxy/server.go`
  - Line 131: cyclomatic complexity 18 for function (*Server).ListenAndServe
- `main.go`
  - Line 1075: cyclomatic complexity 28 for function runProxyMulti
  - Line 146: cyclomatic complexity 26 for function runForward
  - Line 673: cyclomatic complexity 16 for function runProxy
- `internal/proxy/http.go`
  - Line 154: cyclomatic complexity 17 for function (*Server).handlePlainHTTP
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
  - Line 212: "interaktive" is a misspelling of "interactive"
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
  - Line 66: "terminaison" is a misspelling of "termination"
  - Line 67: "terminaison" is a misspelling of "termination"
  - Line 68: "terminaison" is a misspelling of "termination"
  - Line 69: "terminaison" is a misspelling of "termination"
  - Line 70: "terminaison" is a misspelling of "termination"
  - Line 71: "marrage" is a misspelling of "marriage"
  - Line 72: "marrage" is a misspelling of "marriage"
  - Line 73: "marrage" is a misspelling of "marriage"
  - Line 74: "conflit" is a misspelling of "conflict"
  - Line 75: "commandes" is a misspelling of "commands"
  - Line 76: "Konfiguration" is a misspelling of "Configuration"
  - Line 77: "Konfiguration" is a misspelling of "Configuration"
  - Line 78: "Konfiguration" is a misspelling of "Configuration"
  - Line 79: "Konfiguration" is a misspelling of "Configuration"
  - Line 80: "Konfiguration" is a misspelling of "Configuration"
  - Line 81: "interaktive" is a misspelling of "interactive"
  - Line 82: "terminaison" is a misspelling of "termination"
  - Line 83: "terminaison" is a misspelling of "termination"
  - Line 84: "terminaison" is a misspelling of "termination"
  - Line 85: "terminaison" is a misspelling of "termination"
  - Line 86: "terminaison" is a misspelling of "termination"
  - Line 87: "marrage" is a misspelling of "marriage"
  - Line 88: "marrage" is a misspelling of "marriage"
  - Line 89: "marrage" is a misspelling of "marriage"
  - Line 90: "conflit" is a misspelling of "conflict"
  - Line 91: "commandes" is a misspelling of "commands"
  - Line 92: "terminaison" is a misspelling of "termination"
  - Line 93: "terminaison" is a misspelling of "termination"
  - Line 94: "terminaison" is a misspelling of "termination"
  - Line 95: "terminaison" is a misspelling of "termination"
  - Line 96: "terminaison" is a misspelling of "termination"
  - Line 97: "marrage" is a misspelling of "marriage"
  - Line 98: "marrage" is a misspelling of "marriage"
  - Line 99: "marrage" is a misspelling of "marriage"
  - Line 100: "conflit" is a misspelling of "conflict"
  - Line 101: "commandes" is a misspelling of "commands"
  - Line 102: "Konfiguration" is a misspelling of "Configuration"
  - Line 103: "Konfiguration" is a misspelling of "Configuration"
  - Line 104: "Konfiguration" is a misspelling of "Configuration"
  - Line 105: "Konfiguration" is a misspelling of "Configuration"
  - Line 106: "Konfiguration" is a misspelling of "Configuration"
  - Line 107: "marrage" is a misspelling of "marriage"
  - Line 108: "marrage" is a misspelling of "marriage"
  - Line 109: "marrage" is a misspelling of "marriage"
  - Line 110: "commandes" is a misspelling of "commands"
  - Line 111: "Konfiguration" is a misspelling of "Configuration"
  - Line 112: "Konfiguration" is a misspelling of "Configuration"
  - Line 113: "commandes" is a misspelling of "commands"
  - Line 114: "Konfiguration" is a misspelling of "Configuration"
  - Line 115: "commandes" is a misspelling of "commands"
  - Line 116: "Konfiguration" is a misspelling of "Configuration"
  - Line 117: "commandes" is a misspelling of "commands"
  - Line 118: "Konfiguration" is a misspelling of "Configuration"
  - Line 119: "commandes" is a misspelling of "commands"
  - Line 120: "commandes" is a misspelling of "commands"
  - Line 121: "Konfiguration" is a misspelling of "Configuration"
  - Line 122: "terminaison" is a misspelling of "termination"
  - Line 123: "terminaison" is a misspelling of "termination"
  - Line 124: "terminaison" is a misspelling of "termination"
  - Line 125: "terminaison" is a misspelling of "termination"
  - Line 126: "terminaison" is a misspelling of "termination"
  - Line 127: "marrage" is a misspelling of "marriage"
  - Line 128: "marrage" is a misspelling of "marriage"
  - Line 129: "marrage" is a misspelling of "marriage"
  - Line 130: "commandes" is a misspelling of "commands"
  - Line 131: "Konfiguration" is a misspelling of "Configuration"
  - Line 132: "Konfiguration" is a misspelling of "Configuration"
  - Line 133: "Konfiguration" is a misspelling of "Configuration"
  - Line 134: "Konfiguration" is a misspelling of "Configuration"
  - Line 135: "Konfiguration" is a misspelling of "Configuration"
  - Line 136: "interaktive" is a misspelling of "interactive"
  - Line 137: "Konfiguration" is a misspelling of "Configuration"
  - Line 138: "Konfiguration" is a misspelling of "Configuration"
  - Line 139: "Konfiguration" is a misspelling of "Configuration"
  - Line 140: "Konfiguration" is a misspelling of "Configuration"
  - Line 141: "Konfiguration" is a misspelling of "Configuration"
  - Line 142: "interaktive" is a misspelling of "interactive"
  - Line 143: "terminaison" is a misspelling of "termination"
  - Line 144: "terminaison" is a misspelling of "termination"
  - Line 145: "terminaison" is a misspelling of "termination"
  - Line 146: "terminaison" is a misspelling of "termination"
  - Line 147: "terminaison" is a misspelling of "termination"
  - Line 148: "marrage" is a misspelling of "marriage"
  - Line 149: "marrage" is a misspelling of "marriage"
  - Line 150: "marrage" is a misspelling of "marriage"
  - Line 151: "commandes" is a misspelling of "commands"
  - Line 152: "Konfiguration" is a misspelling of "Configuration"
  - Line 153: "Konfiguration" is a misspelling of "Configuration"
  - Line 154: "Konfiguration" is a misspelling of "Configuration"
  - Line 155: "Konfiguration" is a misspelling of "Configuration"
  - Line 156: "Konfiguration" is a misspelling of "Configuration"
  - Line 157: "interaktive" is a misspelling of "interactive"
  - Line 158: "terminaison" is a misspelling of "termination"
  - Line 159: "terminaison" is a misspelling of "termination"
  - Line 160: "terminaison" is a misspelling of "termination"
  - Line 161: "terminaison" is a misspelling of "termination"
  - Line 162: "terminaison" is a misspelling of "termination"
  - Line 163: "marrage" is a misspelling of "marriage"
  - Line 164: "marrage" is a misspelling of "marriage"
  - Line 165: "marrage" is a misspelling of "marriage"
  - Line 166: "commandes" is a misspelling of "commands"
  - Line 167: "Konfiguration" is a misspelling of "Configuration"
  - Line 168: "Konfiguration" is a misspelling of "Configuration"
  - Line 169: "Konfiguration" is a misspelling of "Configuration"
  - Line 170: "Konfiguration" is a misspelling of "Configuration"
  - Line 171: "Konfiguration" is a misspelling of "Configuration"
  - Line 172: "interaktive" is a misspelling of "interactive"
  - Line 173: "terminaison" is a misspelling of "termination"
  - Line 174: "terminaison" is a misspelling of "termination"
  - Line 175: "terminaison" is a misspelling of "termination"
  - Line 176: "terminaison" is a misspelling of "termination"
  - Line 177: "terminaison" is a misspelling of "termination"
  - Line 178: "marrage" is a misspelling of "marriage"
  - Line 179: "marrage" is a misspelling of "marriage"
  - Line 180: "marrage" is a misspelling of "marriage"
  - Line 181: "conflit" is a misspelling of "conflict"
  - Line 182: "commandes" is a misspelling of "commands"
  - Line 183: "Konfiguration" is a misspelling of "Configuration"
  - Line 184: "Konfiguration" is a misspelling of "Configuration"
  - Line 185: "Konfiguration" is a misspelling of "Configuration"
  - Line 186: "Konfiguration" is a misspelling of "Configuration"
  - Line 187: "Konfiguration" is a misspelling of "Configuration"
  - Line 188: "interaktive" is a misspelling of "interactive"
  - Line 189: "Konfiguration" is a misspelling of "Configuration"
  - Line 190: "Konfiguration" is a misspelling of "Configuration"
  - Line 191: "Konfiguration" is a misspelling of "Configuration"
  - Line 192: "Konfiguration" is a misspelling of "Configuration"
  - Line 193: "Konfiguration" is a misspelling of "Configuration"
  - Line 194: "interaktive" is a misspelling of "interactive"
  - Line 195: "terminaison" is a misspelling of "termination"
  - Line 196: "terminaison" is a misspelling of "termination"
  - Line 197: "terminaison" is a misspelling of "termination"
  - Line 198: "terminaison" is a misspelling of "termination"
  - Line 199: "terminaison" is a misspelling of "termination"
  - Line 200: "marrage" is a misspelling of "marriage"
  - Line 201: "marrage" is a misspelling of "marriage"
  - Line 202: "marrage" is a misspelling of "marriage"
  - Line 203: "conflit" is a misspelling of "conflict"
  - Line 204: "commandes" is a misspelling of "commands"
  - Line 206: "Konfiguration" is a misspelling of "Configuration"
  - Line 207: "Konfiguration" is a misspelling of "Configuration"
  - Line 208: "Konfiguration" is a misspelling of "Configuration"
  - Line 209: "Konfiguration" is a misspelling of "Configuration"
  - Line 210: "Konfiguration" is a misspelling of "Configuration"
  - Line 211: "interaktive" is a misspelling of "interactive"
  - Line 213: "terminaison" is a misspelling of "termination"
  - Line 214: "terminaison" is a misspelling of "termination"
  - Line 215: "terminaison" is a misspelling of "termination"
  - Line 216: "terminaison" is a misspelling of "termination"
  - Line 217: "terminaison" is a misspelling of "termination"
  - Line 218: "marrage" is a misspelling of "marriage"
  - Line 219: "marrage" is a misspelling of "marriage"
  - Line 220: "marrage" is a misspelling of "marriage"
  - Line 221: "conflit" is a misspelling of "conflict"
  - Line 222: "commandes" is a misspelling of "commands"

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action) on 2026-09-06 13:10:46 UTC._
