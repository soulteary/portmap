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

- `internal/proxy/socks5.go`
  - Line 62: cyclomatic complexity 18 for function (*Server).handleSOCKS5WithReader
- `main.go`
  - Line 1075: cyclomatic complexity 28 for function runProxyMulti
  - Line 146: cyclomatic complexity 26 for function runForward
  - Line 673: cyclomatic complexity 16 for function runProxy
- `config_test.go`
  - Line 437: cyclomatic complexity 28 for function TestMergeProxyConfigAllFields
  - Line 120: cyclomatic complexity 23 for function TestMergeConfig
  - Line 34: cyclomatic complexity 23 for function TestLoadConfig
- `cmd/loadtest/main.go`
  - Line 64: cyclomatic complexity 25 for function parseFlags
  - Line 636: cyclomatic complexity 16 for function (*worker).runTCP
- `internal/proxy/proxy_test.go`
  - Line 105: cyclomatic complexity 19 for function socks5Dial
  - Line 290: cyclomatic complexity 17 for function TestHTTPProxySanitizesBothDirectionsAndAddsVia
  - Line 359: cyclomatic complexity 16 for function TestHTTPProxyForwardsInformationalResponses
- `internal/proxy/server.go`
  - Line 131: cyclomatic complexity 18 for function (*Server).ListenAndServe
- `internal/proxy/http.go`
  - Line 154: cyclomatic complexity 17 for function (*Server).handlePlainHTTP
- `internal/proxy/events_test.go`
  - Line 106: cyclomatic complexity 17 for function TestProxyRecordsOpenCloseEvents
- `internal/forward/udp.go`
  - Line 45: cyclomatic complexity 17 for function (*Server).serveUDP
- `config.go`
  - Line 340: cyclomatic complexity 45 for function applyProxyConfig
  - Line 452: cyclomatic complexity 36 for function mergeConfig
  - Line 520: cyclomatic complexity 34 for function applyForwardConfig
- `coverage_test.go`
  - Line 158: cyclomatic complexity 24 for function TestBuildProxyUpstream
- `internal/forward/events_test.go`
  - Line 29: cyclomatic complexity 21 for function TestForwardRecordsOpenCloseEvents

### misspell

- `.github/goreportcard-report.md`
  - Line 62: "Konfiguration" is a misspelling of "Configuration"
  - Line 63: "Konfiguration" is a misspelling of "Configuration"
  - Line 64: "Konfiguration" is a misspelling of "Configuration"
  - Line 65: "Konfiguration" is a misspelling of "Configuration"
  - Line 66: "Konfiguration" is a misspelling of "Configuration"
  - Line 67: "interaktive" is a misspelling of "interactive"
  - Line 69: "terminaison" is a misspelling of "termination"
  - Line 70: "terminaison" is a misspelling of "termination"
  - Line 71: "terminaison" is a misspelling of "termination"
  - Line 72: "terminaison" is a misspelling of "termination"
  - Line 73: "terminaison" is a misspelling of "termination"
  - Line 74: "marrage" is a misspelling of "marriage"
  - Line 75: "marrage" is a misspelling of "marriage"
  - Line 76: "marrage" is a misspelling of "marriage"
  - Line 77: "conflit" is a misspelling of "conflict"
  - Line 78: "commandes" is a misspelling of "commands"
  - Line 80: "Konfiguration" is a misspelling of "Configuration"
  - Line 81: "Konfiguration" is a misspelling of "Configuration"
  - Line 82: "Konfiguration" is a misspelling of "Configuration"
  - Line 83: "Konfiguration" is a misspelling of "Configuration"
  - Line 84: "Konfiguration" is a misspelling of "Configuration"
  - Line 85: "interaktive" is a misspelling of "interactive"
  - Line 86: "terminaison" is a misspelling of "termination"
  - Line 87: "terminaison" is a misspelling of "termination"
  - Line 88: "terminaison" is a misspelling of "termination"
  - Line 89: "terminaison" is a misspelling of "termination"
  - Line 90: "terminaison" is a misspelling of "termination"
  - Line 91: "marrage" is a misspelling of "marriage"
  - Line 92: "marrage" is a misspelling of "marriage"
  - Line 93: "marrage" is a misspelling of "marriage"
  - Line 94: "conflit" is a misspelling of "conflict"
  - Line 95: "commandes" is a misspelling of "commands"
  - Line 96: "Konfiguration" is a misspelling of "Configuration"
  - Line 97: "Konfiguration" is a misspelling of "Configuration"
  - Line 98: "Konfiguration" is a misspelling of "Configuration"
  - Line 99: "Konfiguration" is a misspelling of "Configuration"
  - Line 100: "Konfiguration" is a misspelling of "Configuration"
  - Line 101: "interaktive" is a misspelling of "interactive"
  - Line 102: "terminaison" is a misspelling of "termination"
  - Line 103: "terminaison" is a misspelling of "termination"
  - Line 104: "terminaison" is a misspelling of "termination"
  - Line 105: "terminaison" is a misspelling of "termination"
  - Line 106: "terminaison" is a misspelling of "termination"
  - Line 107: "marrage" is a misspelling of "marriage"
  - Line 108: "marrage" is a misspelling of "marriage"
  - Line 109: "marrage" is a misspelling of "marriage"
  - Line 110: "conflit" is a misspelling of "conflict"
  - Line 111: "commandes" is a misspelling of "commands"
  - Line 112: "terminaison" is a misspelling of "termination"
  - Line 113: "terminaison" is a misspelling of "termination"
  - Line 114: "terminaison" is a misspelling of "termination"
  - Line 115: "terminaison" is a misspelling of "termination"
  - Line 116: "terminaison" is a misspelling of "termination"
  - Line 117: "marrage" is a misspelling of "marriage"
  - Line 118: "marrage" is a misspelling of "marriage"
  - Line 119: "marrage" is a misspelling of "marriage"
  - Line 120: "conflit" is a misspelling of "conflict"
  - Line 121: "commandes" is a misspelling of "commands"
  - Line 122: "Konfiguration" is a misspelling of "Configuration"
  - Line 123: "Konfiguration" is a misspelling of "Configuration"
  - Line 124: "Konfiguration" is a misspelling of "Configuration"
  - Line 125: "Konfiguration" is a misspelling of "Configuration"
  - Line 126: "Konfiguration" is a misspelling of "Configuration"
  - Line 127: "marrage" is a misspelling of "marriage"
  - Line 128: "marrage" is a misspelling of "marriage"
  - Line 129: "marrage" is a misspelling of "marriage"
  - Line 130: "commandes" is a misspelling of "commands"
  - Line 131: "Konfiguration" is a misspelling of "Configuration"
  - Line 132: "Konfiguration" is a misspelling of "Configuration"
  - Line 133: "commandes" is a misspelling of "commands"
  - Line 134: "Konfiguration" is a misspelling of "Configuration"
  - Line 135: "commandes" is a misspelling of "commands"
  - Line 136: "Konfiguration" is a misspelling of "Configuration"
  - Line 137: "commandes" is a misspelling of "commands"
  - Line 138: "Konfiguration" is a misspelling of "Configuration"
  - Line 139: "commandes" is a misspelling of "commands"
  - Line 140: "commandes" is a misspelling of "commands"
  - Line 141: "Konfiguration" is a misspelling of "Configuration"
  - Line 142: "terminaison" is a misspelling of "termination"
  - Line 143: "terminaison" is a misspelling of "termination"
  - Line 144: "terminaison" is a misspelling of "termination"
  - Line 145: "terminaison" is a misspelling of "termination"
  - Line 146: "terminaison" is a misspelling of "termination"
  - Line 147: "marrage" is a misspelling of "marriage"
  - Line 148: "marrage" is a misspelling of "marriage"
  - Line 149: "marrage" is a misspelling of "marriage"
  - Line 150: "commandes" is a misspelling of "commands"
  - Line 151: "Konfiguration" is a misspelling of "Configuration"
  - Line 152: "Konfiguration" is a misspelling of "Configuration"
  - Line 153: "Konfiguration" is a misspelling of "Configuration"
  - Line 154: "Konfiguration" is a misspelling of "Configuration"
  - Line 155: "Konfiguration" is a misspelling of "Configuration"
  - Line 156: "interaktive" is a misspelling of "interactive"
  - Line 157: "Konfiguration" is a misspelling of "Configuration"
  - Line 158: "Konfiguration" is a misspelling of "Configuration"
  - Line 159: "Konfiguration" is a misspelling of "Configuration"
  - Line 160: "Konfiguration" is a misspelling of "Configuration"
  - Line 161: "Konfiguration" is a misspelling of "Configuration"
  - Line 162: "interaktive" is a misspelling of "interactive"
  - Line 163: "terminaison" is a misspelling of "termination"
  - Line 164: "terminaison" is a misspelling of "termination"
  - Line 165: "terminaison" is a misspelling of "termination"
  - Line 166: "terminaison" is a misspelling of "termination"
  - Line 167: "terminaison" is a misspelling of "termination"
  - Line 168: "marrage" is a misspelling of "marriage"
  - Line 169: "marrage" is a misspelling of "marriage"
  - Line 170: "marrage" is a misspelling of "marriage"
  - Line 171: "commandes" is a misspelling of "commands"
  - Line 172: "Konfiguration" is a misspelling of "Configuration"
  - Line 173: "Konfiguration" is a misspelling of "Configuration"
  - Line 174: "Konfiguration" is a misspelling of "Configuration"
  - Line 175: "Konfiguration" is a misspelling of "Configuration"
  - Line 176: "Konfiguration" is a misspelling of "Configuration"
  - Line 177: "interaktive" is a misspelling of "interactive"
  - Line 178: "terminaison" is a misspelling of "termination"
  - Line 179: "terminaison" is a misspelling of "termination"
  - Line 180: "terminaison" is a misspelling of "termination"
  - Line 181: "terminaison" is a misspelling of "termination"
  - Line 182: "terminaison" is a misspelling of "termination"
  - Line 183: "marrage" is a misspelling of "marriage"
  - Line 184: "marrage" is a misspelling of "marriage"
  - Line 185: "marrage" is a misspelling of "marriage"
  - Line 186: "commandes" is a misspelling of "commands"
  - Line 187: "Konfiguration" is a misspelling of "Configuration"
  - Line 188: "Konfiguration" is a misspelling of "Configuration"
  - Line 189: "Konfiguration" is a misspelling of "Configuration"
  - Line 190: "Konfiguration" is a misspelling of "Configuration"
  - Line 191: "Konfiguration" is a misspelling of "Configuration"
  - Line 192: "interaktive" is a misspelling of "interactive"
  - Line 193: "terminaison" is a misspelling of "termination"
  - Line 194: "terminaison" is a misspelling of "termination"
  - Line 195: "terminaison" is a misspelling of "termination"
  - Line 196: "terminaison" is a misspelling of "termination"
  - Line 197: "terminaison" is a misspelling of "termination"
  - Line 198: "marrage" is a misspelling of "marriage"
  - Line 199: "marrage" is a misspelling of "marriage"
  - Line 200: "marrage" is a misspelling of "marriage"
  - Line 201: "conflit" is a misspelling of "conflict"
  - Line 202: "commandes" is a misspelling of "commands"
  - Line 203: "Konfiguration" is a misspelling of "Configuration"
  - Line 204: "Konfiguration" is a misspelling of "Configuration"
  - Line 205: "Konfiguration" is a misspelling of "Configuration"
  - Line 206: "Konfiguration" is a misspelling of "Configuration"
  - Line 207: "Konfiguration" is a misspelling of "Configuration"
  - Line 208: "interaktive" is a misspelling of "interactive"
  - Line 209: "Konfiguration" is a misspelling of "Configuration"
  - Line 210: "Konfiguration" is a misspelling of "Configuration"
  - Line 211: "Konfiguration" is a misspelling of "Configuration"
  - Line 212: "Konfiguration" is a misspelling of "Configuration"
  - Line 213: "Konfiguration" is a misspelling of "Configuration"
  - Line 214: "interaktive" is a misspelling of "interactive"
  - Line 215: "terminaison" is a misspelling of "termination"
  - Line 216: "terminaison" is a misspelling of "termination"
  - Line 217: "terminaison" is a misspelling of "termination"
  - Line 218: "terminaison" is a misspelling of "termination"
  - Line 219: "terminaison" is a misspelling of "termination"
  - Line 220: "marrage" is a misspelling of "marriage"
  - Line 221: "marrage" is a misspelling of "marriage"
  - Line 222: "marrage" is a misspelling of "marriage"
  - Line 223: "conflit" is a misspelling of "conflict"
  - Line 224: "commandes" is a misspelling of "commands"
  - Line 225: "Konfiguration" is a misspelling of "Configuration"
  - Line 226: "Konfiguration" is a misspelling of "Configuration"
  - Line 227: "Konfiguration" is a misspelling of "Configuration"
  - Line 228: "Konfiguration" is a misspelling of "Configuration"
  - Line 229: "Konfiguration" is a misspelling of "Configuration"
  - Line 230: "interaktive" is a misspelling of "interactive"
  - Line 231: "terminaison" is a misspelling of "termination"
  - Line 232: "terminaison" is a misspelling of "termination"
  - Line 233: "terminaison" is a misspelling of "termination"
  - Line 234: "terminaison" is a misspelling of "termination"
  - Line 235: "terminaison" is a misspelling of "termination"
  - Line 236: "marrage" is a misspelling of "marriage"
  - Line 237: "marrage" is a misspelling of "marriage"
  - Line 238: "marrage" is a misspelling of "marriage"
  - Line 239: "conflit" is a misspelling of "conflict"
  - Line 240: "commandes" is a misspelling of "commands"
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
- `internal/i18n/messages_de.go`
  - Line 51: "Konfiguration" is a misspelling of "Configuration"
  - Line 100: "Konfiguration" is a misspelling of "Configuration"
  - Line 101: "Konfiguration" is a misspelling of "Configuration"
  - Line 104: "Konfiguration" is a misspelling of "Configuration"
  - Line 107: "Konfiguration" is a misspelling of "Configuration"
  - Line 212: "interaktive" is a misspelling of "interactive"

---

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action) on 2026-09-06 13:41:11 UTC._
