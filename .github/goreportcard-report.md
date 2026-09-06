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
- `internal/proxy/http.go`
  - Line 154: cyclomatic complexity 17 for function (*Server).handlePlainHTTP
- `main.go`
  - Line 1080: cyclomatic complexity 28 for function runProxyMulti
  - Line 151: cyclomatic complexity 26 for function runForward
  - Line 678: cyclomatic complexity 16 for function runProxy
- `internal/proxy/server.go`
  - Line 131: cyclomatic complexity 18 for function (*Server).ListenAndServe
- `internal/proxy/events_test.go`
  - Line 106: cyclomatic complexity 17 for function TestProxyRecordsOpenCloseEvents
- `internal/forward/udp.go`
  - Line 45: cyclomatic complexity 17 for function (*Server).serveUDP
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

### misspell

- `.github/goreportcard-report.md`
  - Line 62: "Konfiguration" is a misspelling of "Configuration"
  - Line 63: "Konfiguration" is a misspelling of "Configuration"
  - Line 64: "Konfiguration" is a misspelling of "Configuration"
  - Line 65: "Konfiguration" is a misspelling of "Configuration"
  - Line 66: "Konfiguration" is a misspelling of "Configuration"
  - Line 67: "interaktive" is a misspelling of "interactive"
  - Line 68: "terminaison" is a misspelling of "termination"
  - Line 69: "terminaison" is a misspelling of "termination"
  - Line 70: "terminaison" is a misspelling of "termination"
  - Line 71: "terminaison" is a misspelling of "termination"
  - Line 72: "terminaison" is a misspelling of "termination"
  - Line 73: "marrage" is a misspelling of "marriage"
  - Line 74: "marrage" is a misspelling of "marriage"
  - Line 75: "marrage" is a misspelling of "marriage"
  - Line 76: "conflit" is a misspelling of "conflict"
  - Line 77: "commandes" is a misspelling of "commands"
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
  - Line 108: "conflit" is a misspelling of "conflict"
  - Line 109: "commandes" is a misspelling of "commands"
  - Line 110: "terminaison" is a misspelling of "termination"
  - Line 111: "terminaison" is a misspelling of "termination"
  - Line 112: "terminaison" is a misspelling of "termination"
  - Line 113: "terminaison" is a misspelling of "termination"
  - Line 114: "terminaison" is a misspelling of "termination"
  - Line 115: "marrage" is a misspelling of "marriage"
  - Line 116: "marrage" is a misspelling of "marriage"
  - Line 117: "marrage" is a misspelling of "marriage"
  - Line 118: "conflit" is a misspelling of "conflict"
  - Line 119: "commandes" is a misspelling of "commands"
  - Line 120: "Konfiguration" is a misspelling of "Configuration"
  - Line 121: "Konfiguration" is a misspelling of "Configuration"
  - Line 122: "Konfiguration" is a misspelling of "Configuration"
  - Line 123: "Konfiguration" is a misspelling of "Configuration"
  - Line 124: "Konfiguration" is a misspelling of "Configuration"
  - Line 125: "marrage" is a misspelling of "marriage"
  - Line 126: "marrage" is a misspelling of "marriage"
  - Line 127: "marrage" is a misspelling of "marriage"
  - Line 128: "commandes" is a misspelling of "commands"
  - Line 129: "Konfiguration" is a misspelling of "Configuration"
  - Line 130: "Konfiguration" is a misspelling of "Configuration"
  - Line 131: "commandes" is a misspelling of "commands"
  - Line 132: "Konfiguration" is a misspelling of "Configuration"
  - Line 133: "commandes" is a misspelling of "commands"
  - Line 134: "Konfiguration" is a misspelling of "Configuration"
  - Line 135: "commandes" is a misspelling of "commands"
  - Line 136: "Konfiguration" is a misspelling of "Configuration"
  - Line 137: "commandes" is a misspelling of "commands"
  - Line 138: "commandes" is a misspelling of "commands"
  - Line 139: "Konfiguration" is a misspelling of "Configuration"
  - Line 140: "terminaison" is a misspelling of "termination"
  - Line 141: "terminaison" is a misspelling of "termination"
  - Line 142: "terminaison" is a misspelling of "termination"
  - Line 143: "terminaison" is a misspelling of "termination"
  - Line 144: "terminaison" is a misspelling of "termination"
  - Line 145: "marrage" is a misspelling of "marriage"
  - Line 146: "marrage" is a misspelling of "marriage"
  - Line 147: "marrage" is a misspelling of "marriage"
  - Line 148: "commandes" is a misspelling of "commands"
  - Line 149: "Konfiguration" is a misspelling of "Configuration"
  - Line 150: "Konfiguration" is a misspelling of "Configuration"
  - Line 151: "Konfiguration" is a misspelling of "Configuration"
  - Line 152: "Konfiguration" is a misspelling of "Configuration"
  - Line 153: "Konfiguration" is a misspelling of "Configuration"
  - Line 154: "interaktive" is a misspelling of "interactive"
  - Line 155: "Konfiguration" is a misspelling of "Configuration"
  - Line 156: "Konfiguration" is a misspelling of "Configuration"
  - Line 157: "Konfiguration" is a misspelling of "Configuration"
  - Line 158: "Konfiguration" is a misspelling of "Configuration"
  - Line 159: "Konfiguration" is a misspelling of "Configuration"
  - Line 160: "interaktive" is a misspelling of "interactive"
  - Line 161: "terminaison" is a misspelling of "termination"
  - Line 162: "terminaison" is a misspelling of "termination"
  - Line 163: "terminaison" is a misspelling of "termination"
  - Line 164: "terminaison" is a misspelling of "termination"
  - Line 165: "terminaison" is a misspelling of "termination"
  - Line 166: "marrage" is a misspelling of "marriage"
  - Line 167: "marrage" is a misspelling of "marriage"
  - Line 168: "marrage" is a misspelling of "marriage"
  - Line 169: "commandes" is a misspelling of "commands"
  - Line 170: "Konfiguration" is a misspelling of "Configuration"
  - Line 171: "Konfiguration" is a misspelling of "Configuration"
  - Line 172: "Konfiguration" is a misspelling of "Configuration"
  - Line 173: "Konfiguration" is a misspelling of "Configuration"
  - Line 174: "Konfiguration" is a misspelling of "Configuration"
  - Line 175: "interaktive" is a misspelling of "interactive"
  - Line 176: "terminaison" is a misspelling of "termination"
  - Line 177: "terminaison" is a misspelling of "termination"
  - Line 178: "terminaison" is a misspelling of "termination"
  - Line 179: "terminaison" is a misspelling of "termination"
  - Line 180: "terminaison" is a misspelling of "termination"
  - Line 181: "marrage" is a misspelling of "marriage"
  - Line 182: "marrage" is a misspelling of "marriage"
  - Line 183: "marrage" is a misspelling of "marriage"
  - Line 184: "commandes" is a misspelling of "commands"
  - Line 185: "Konfiguration" is a misspelling of "Configuration"
  - Line 186: "Konfiguration" is a misspelling of "Configuration"
  - Line 187: "Konfiguration" is a misspelling of "Configuration"
  - Line 188: "Konfiguration" is a misspelling of "Configuration"
  - Line 189: "Konfiguration" is a misspelling of "Configuration"
  - Line 190: "interaktive" is a misspelling of "interactive"
  - Line 191: "terminaison" is a misspelling of "termination"
  - Line 192: "terminaison" is a misspelling of "termination"
  - Line 193: "terminaison" is a misspelling of "termination"
  - Line 194: "terminaison" is a misspelling of "termination"
  - Line 195: "terminaison" is a misspelling of "termination"
  - Line 196: "marrage" is a misspelling of "marriage"
  - Line 197: "marrage" is a misspelling of "marriage"
  - Line 198: "marrage" is a misspelling of "marriage"
  - Line 199: "conflit" is a misspelling of "conflict"
  - Line 200: "commandes" is a misspelling of "commands"
  - Line 201: "Konfiguration" is a misspelling of "Configuration"
  - Line 202: "Konfiguration" is a misspelling of "Configuration"
  - Line 203: "Konfiguration" is a misspelling of "Configuration"
  - Line 204: "Konfiguration" is a misspelling of "Configuration"
  - Line 205: "Konfiguration" is a misspelling of "Configuration"
  - Line 206: "interaktive" is a misspelling of "interactive"
  - Line 207: "Konfiguration" is a misspelling of "Configuration"
  - Line 208: "Konfiguration" is a misspelling of "Configuration"
  - Line 209: "Konfiguration" is a misspelling of "Configuration"
  - Line 210: "Konfiguration" is a misspelling of "Configuration"
  - Line 211: "Konfiguration" is a misspelling of "Configuration"
  - Line 212: "interaktive" is a misspelling of "interactive"
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
  - Line 223: "Konfiguration" is a misspelling of "Configuration"
  - Line 224: "Konfiguration" is a misspelling of "Configuration"
  - Line 225: "Konfiguration" is a misspelling of "Configuration"
  - Line 226: "Konfiguration" is a misspelling of "Configuration"
  - Line 227: "Konfiguration" is a misspelling of "Configuration"
  - Line 228: "interaktive" is a misspelling of "interactive"
  - Line 229: "terminaison" is a misspelling of "termination"
  - Line 230: "terminaison" is a misspelling of "termination"
  - Line 231: "terminaison" is a misspelling of "termination"
  - Line 232: "terminaison" is a misspelling of "termination"
  - Line 233: "terminaison" is a misspelling of "termination"
  - Line 234: "marrage" is a misspelling of "marriage"
  - Line 235: "marrage" is a misspelling of "marriage"
  - Line 236: "marrage" is a misspelling of "marriage"
  - Line 237: "conflit" is a misspelling of "conflict"
  - Line 238: "commandes" is a misspelling of "commands"
  - Line 240: "terminaison" is a misspelling of "termination"
  - Line 241: "terminaison" is a misspelling of "termination"
  - Line 242: "terminaison" is a misspelling of "termination"
  - Line 243: "terminaison" is a misspelling of "termination"
  - Line 244: "terminaison" is a misspelling of "termination"
  - Line 245: "marrage" is a misspelling of "marriage"
  - Line 246: "marrage" is a misspelling of "marriage"
  - Line 247: "marrage" is a misspelling of "marriage"
  - Line 248: "conflit" is a misspelling of "conflict"
  - Line 249: "commandes" is a misspelling of "commands"
  - Line 251: "Konfiguration" is a misspelling of "Configuration"
  - Line 252: "Konfiguration" is a misspelling of "Configuration"
  - Line 253: "Konfiguration" is a misspelling of "Configuration"
  - Line 254: "Konfiguration" is a misspelling of "Configuration"
  - Line 255: "Konfiguration" is a misspelling of "Configuration"
  - Line 256: "interaktive" is a misspelling of "interactive"
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

_Generated by [Go Report Card](https://github.com/soulteary/goreportcard-action) on 2026-09-06 13:42:45 UTC._
