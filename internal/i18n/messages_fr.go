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

var messagesFR = map[string]string{
	KeyUsageTitle:  "portmap - redirection de ports TCP/UDP (équivalent socat)",
	KeyUsageLine:   "Utilisation : %s [flags]\n\nÉquivalent à : sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222\n\nflags :",
	KeyVersionLine: "portmap %s (commit %s, built %s)",

	KeyFlagListenPort:  "port d'écoute local",
	KeyFlagListenHost:  "adresse d'écoute locale (par défaut : toutes les interfaces)",
	KeyFlagTarget:      "adresse cible de redirection host:port",
	KeyFlagMode:        "mode de redirection : go (implémentation Go pure) ou socat (invoque le socat système)",
	KeyFlagProto:       "protocole de redirection : tcp ou udp",
	KeyFlagReuseAddr:   "activer SO_REUSEADDR",
	KeyFlagSudo:        "exécuter via sudo en mode socat",
	KeyFlagDialTimeout: "délai de connexion vers la cible",
	KeyFlagMaxConns:    "nombre max de connexions simultanées, 0 signifie illimité (mode go uniquement ; limite les sessions simultanées en UDP)",
	KeyFlagIdleTimeout: "délai d'inactivité, déconnexion si aucune donnée dans les deux sens, 0 désactive (mode go uniquement ; en UDP, 0 signifie recyclage des sessions par défaut à 60 s)",
	KeyFlagLogLevel:    "niveau de journalisation : info ou debug (mode go uniquement)",
	KeyFlagQuiet:       "mode silencieux, supprime les journaux courants par connexion (mode go uniquement)",
	KeyFlagVersion:     "afficher les informations de version et quitter",
	KeyFlagConfig:      "chemin du fichier de configuration YAML",
	KeyFlagLang:        "langue de l'interface (%s) ; détectée automatiquement depuis le système par défaut",

	KeyErrListenPort:  "port d'écoute invalide : %d",
	KeyErrTargetEmpty: "target ne doit pas être vide",
	KeyErrProto:       "proto inconnu : %q (choisir tcp ou udp)",
	KeyErrIdleNeg:     "idle-timeout ne doit pas être négatif : %s",
	KeyErrMaxConnsNeg: "max-conns ne doit pas être négatif : %d",
	KeyErrDialNeg:     "dial-timeout ne doit pas être négatif : %s",
	KeyErrLogLevel:    "log-level inconnu : %q (choisir info ou debug)",
	KeyErrMode:        "mode inconnu : %q (choisir go ou socat)",
	KeyErrServeExit:   "le service de redirection s'est arrêté : %w",
	KeyErrSocatFailed: "l'exécution de socat a échoué : %w",

	KeyLogEffectiveConfig: "configuration effective : %s",
	KeyLogSocatIgnore:     "note : le mode socat ignore les paramètres suivants réservés au mode go : %s",
	KeyLogSocatExec:       "exécution : %s",
	KeyLogStatus:          "status: active=%d total=%d",

	KeyErrConfigRead:  "échec de lecture du fichier de configuration : %w",
	KeyErrConfigParse: "échec d'analyse du fichier de configuration : %w",
	KeyErrConfigDial:  "dial_timeout invalide dans le fichier de configuration : %w",
	KeyErrConfigIdle:  "idle_timeout invalide dans le fichier de configuration : %w",

	KeyErrUnsupportedNet:  "réseau non pris en charge : %q",
	KeyLogTCPListening:    "écoute sur %s (tcp), redirection vers %s (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogDialFailed:      "échec de connexion à %s : %v",
	KeyLogConnOpen:        "[#%d] ouverture %s <-> %s (active=%d)",
	KeyLogConnClose:       "[#%d] fermeture %s <-> %s (up=%dB down=%dB dur=%s)",
	KeyLogPipeError:       "[#%d] erreur de redirection %s : %v",
	KeyLogUDPListening:    "écoute sur %s (udp), redirection vers %s (reuseaddr=%v, max-conns=%d, idle=%s)",
	KeyLogUDPLimit:        "limite de sessions udp atteinte, paquet de %s ignoré",
	KeyLogUDPDialFailed:   "échec de connexion udp à %s : %v",
	KeyLogUDPWriteTarget:  "échec d'écriture udp vers la cible : %v",
	KeyLogUDPSessionOpen:  "[#%d] session udp %s <-> %s (active=%d)",
	KeyLogUDPSessionClose: "[#%d] session udp fermée %s",
	KeyLogUDPWriteClient:  "échec d'écriture udp vers le client : %v",

	KeyErrSocatProto:      "proto invalide : %q",
	KeyErrSocatPort:       "port d'écoute invalide : %d",
	KeyErrSocatTarget:     "target vide",
	KeyErrSocatNotFound:   "%q introuvable dans le PATH : %w",
	KeyErrSocatInvalidStr: "<options socat invalides>",
}
