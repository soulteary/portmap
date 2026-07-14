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

var messagesDE = map[string]string{
	KeyUsageTitle:  "portmap - TCP/UDP-Portweiterleitung (socat-Äquivalent)",
	KeyUsageLine:   "Verwendung: %s [flags]\n\nEntspricht: sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222\n\nflags:",
	KeyVersionLine: "portmap %s (commit %s, built %s)",

	KeyFlagListenPort:  "lokaler Lausch-Port",
	KeyFlagListenHost:  "lokale Lausch-Adresse (Standard: alle Schnittstellen)",
	KeyFlagTarget:      "Weiterleitungsziel host:port",
	KeyFlagMode:        "Weiterleitungsmodus: go (reine Go-Implementierung) oder socat (System-socat aufrufen)",
	KeyFlagProto:       "Weiterleitungsprotokoll: tcp oder udp",
	KeyFlagReuseAddr:   "SO_REUSEADDR aktivieren",
	KeyFlagSudo:        "im socat-Modus über sudo ausführen",
	KeyFlagDialTimeout: "Verbindungs-Timeout zum Ziel",
	KeyFlagMaxConns:    "maximale gleichzeitige Verbindungen, 0 bedeutet unbegrenzt (nur go-Modus; bei UDP werden gleichzeitige Sitzungen begrenzt)",
	KeyFlagIdleTimeout: "Leerlauf-Timeout, Trennung bei beidseitiger Untätigkeit, 0 deaktiviert (nur go-Modus; bei UDP bedeutet 0 standardmäßig 60 s Sitzungsbereinigung)",
	KeyFlagLogLevel:    "Protokollstufe: info oder debug (nur go-Modus)",
	KeyFlagQuiet:       "stiller Modus, unterdrückt reguläre Logs pro Verbindung (nur go-Modus)",
	KeyFlagVersion:     "Versionsinformationen ausgeben und beenden",
	KeyFlagConfig:      "Pfad zur YAML-Konfigurationsdatei",
	KeyFlagLang:        "Oberflächensprache (%s); standardmäßig automatisch vom System erkannt",

	KeyErrListenPort:  "ungültiger Lausch-Port: %d",
	KeyErrTargetEmpty: "target darf nicht leer sein",
	KeyErrProto:       "unbekanntes proto: %q (tcp oder udp wählen)",
	KeyErrIdleNeg:     "idle-timeout darf nicht negativ sein: %s",
	KeyErrMaxConnsNeg: "max-conns darf nicht negativ sein: %d",
	KeyErrDialNeg:     "dial-timeout darf nicht negativ sein: %s",
	KeyErrLogLevel:    "unbekanntes log-level: %q (info oder debug wählen)",
	KeyErrMode:        "unbekannter mode: %q (go oder socat wählen)",
	KeyErrServeExit:   "Weiterleitungsdienst beendet: %w",
	KeyErrSocatFailed: "socat-Ausführung fehlgeschlagen: %w",

	KeyLogEffectiveConfig: "wirksame Konfiguration: %s",
	KeyLogSocatIgnore:     "Hinweis: der socat-Modus ignoriert die folgenden nur im go-Modus unterstützten Parameter: %s",
	KeyLogSocatExec:       "Ausführung: %s",
	KeyLogStatus:          "status: active=%d total=%d",

	KeyErrConfigRead:  "Konfigurationsdatei konnte nicht gelesen werden: %w",
	KeyErrConfigParse: "Konfigurationsdatei konnte nicht geparst werden: %w",
	KeyErrConfigDial:  "ungültiges dial_timeout in der Konfigurationsdatei: %w",
	KeyErrConfigIdle:  "ungültiges idle_timeout in der Konfigurationsdatei: %w",

	KeyErrUnsupportedNet:  "nicht unterstütztes Netzwerk: %q",
	KeyLogTCPListening:    "lausche auf %s (tcp), leite weiter an %s (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogDialFailed:      "Verbindung zu %s fehlgeschlagen: %v",
	KeyLogConnOpen:        "[#%d] geöffnet %s <-> %s (active=%d)",
	KeyLogConnClose:       "[#%d] geschlossen %s <-> %s (up=%dB down=%dB dur=%s)",
	KeyLogPipeError:       "[#%d] Weiterleitungsfehler %s: %v",
	KeyLogUDPListening:    "lausche auf %s (udp), leite weiter an %s (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogUDPLimit:        "udp-Sitzungslimit erreicht, Paket von %s verworfen",
	KeyLogUDPDialFailed:   "udp-Verbindung zu %s fehlgeschlagen: %v",
	KeyLogUDPWriteTarget:  "udp-Schreiben zum Ziel fehlgeschlagen: %v",
	KeyLogUDPSessionOpen:  "[#%d] udp-Sitzung %s <-> %s (active=%d)",
	KeyLogUDPSessionClose: "[#%d] udp-Sitzung geschlossen %s",
	KeyLogUDPWriteClient:  "udp-Schreiben zum Client fehlgeschlagen: %v",

	KeyErrSocatProto:      "ungültiges proto: %q",
	KeyErrSocatPort:       "ungültiger Lausch-Port: %d",
	KeyErrSocatTarget:     "leeres target",
	KeyErrSocatNotFound:   "%q nicht im PATH gefunden: %w",
	KeyErrSocatInvalidStr: "<ungültige socat-Optionen>",
}
