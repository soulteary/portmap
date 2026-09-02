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

	KeyErrConfigRead:      "échec de lecture du fichier de configuration : %w",
	KeyErrConfigParse:     "échec d'analyse du fichier de configuration : %w",
	KeyErrConfigDial:      "dial_timeout invalide dans le fichier de configuration : %w",
	KeyErrConfigIdle:      "idle_timeout invalide dans le fichier de configuration : %w",
	KeyErrConfigHandshake: "handshake_timeout invalide dans le fichier de configuration : %w",

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

	KeyUsageSubcommands: "sous-commandes :\n  forward   redirection de ports TCP/UDP (par défaut)\n  proxy     proxy SOCKS5 + HTTP sur un seul port\n  version   afficher les informations de version",
	KeyErrUnknownSub:    "sous-commande inconnue : %q (choisir forward, proxy ou version)",

	KeyProxyUsageTitle: "portmap proxy - proxy SOCKS5 + HTTP sur un seul port",
	KeyProxyUsageLine:  "Utilisation : %s proxy [flags]\n\nUn unique port d'écoute détecte automatiquement les clients SOCKS5 et HTTP/HTTPS ; les connexions sortantes se connectent en direct par défaut, ou sont transférées via l'amont configuré (SOCKS5/HTTP/SSH). Les proxys d'environnement (HTTP_PROXY/HTTPS_PROXY/ALL_PROXY) sont toujours ignorés.\n\nflags :",

	KeyFlagProxyAddr:             "adresse d'écoute, partagée par SOCKS5 et HTTP",
	KeyFlagProxyDialTimeout:      "délai de connexion sortante",
	KeyFlagProxyMaxConns:         "nombre maximal de connexions proxy simultanées, 0 = illimité",
	KeyFlagProxyHandshakeTimeout: "délai de négociation du protocole, 0 = désactivé",
	KeyFlagProxyIdleTimeout:      "délai d'inactivité bidirectionnel, 0 = désactivé",
	KeyFlagProxyAllowPublic:      "autoriser l'écoute hors boucle locale (aucune authentification)",

	KeyFlagProxyUpstream:           "URL du proxy amont pour les connexions sortantes, ex. socks5://user:pass@host:1080, http://host:3128, ssh://user@host:22 (vide = connexion directe)",
	KeyFlagProxyUpstreamIdentity:   "fichier de clé privée SSH pour l'authentification de l'amont ssh",
	KeyFlagProxyUpstreamKnownHosts: "fichier SSH known_hosts pour la vérification de la clé d'hôte (par défaut ~/.ssh/known_hosts)",
	KeyFlagProxyUpstreamInsecure:   "ignorer la vérification de la clé d'hôte SSH amont (non sécurisé, uniquement pour les environnements de test auto-hébergés)",

	KeyLogProxyStarted:        "proxy démarré, écoute sur %s (SOCKS5 + HTTP, proxys d'environnement ignorés)",
	KeyLogProxyAcceptFailed:   "échec d'acceptation de la connexion : %v",
	KeyLogProxyDetectFailed:   "échec de détection du protocole (%s) : %v",
	KeyLogProxySOCKS5Failed:   "erreur de traitement SOCKS5 (%s) : %v",
	KeyLogProxyHTTPFailed:     "erreur de traitement HTTP (%s) : %v",
	KeyLogProxySOCKS5Relay:    "SOCKS5 %s -> %s",
	KeyLogProxyHTTPConnect:    "HTTP CONNECT %s -> %s",
	KeyLogProxyHTTPPlain:      "HTTP %s %s -> %s",
	KeyLogProxyShuttingDown:   "signal d'arrêt reçu, fermeture...",
	KeyLogProxyShutdownFailed: "l'arrêt gracieux n'a pas abouti : %v",
	KeyLogProxyConnLimit:      "rejet de %s : limite de connexions atteinte (%d)",

	KeyLogProxyUpstreamEnabled:      "proxy amont activé : %s %s",
	KeyLogProxyUpstreamInsecure:     "AVERTISSEMENT : la vérification de la clé d'hôte SSH amont est désactivée (-upstream-insecure) ; la connexion est vulnérable aux attaques de l'intercepteur",
	KeyLogProxyUpstreamSSHConnect:   "amont SSH connecté : %s",
	KeyLogProxyUpstreamSSHReconnect: "connexion amont SSH perdue, reconnexion : %s",

	KeyErrProxyExit:         "le service proxy s'est arrêté : %w",
	KeyErrProxyHandshakeNeg: "handshake-timeout ne peut pas être négatif : %s",
	KeyErrProxyPublicListen: "refus du proxy non authentifié sur l'adresse publique %s ; utilisez -allow-public pour l'autoriser",
	KeyErrProxySelfTarget:   "cible proxy %s refusée car elle correspond à cette écoute",

	KeyErrProxySocksReadNMethods: "échec de lecture de NMETHODS : %w",
	KeyErrProxySocksReadMethods:  "échec de lecture de METHODS : %w",
	KeyErrProxySocksNoAuth:       "le client ne prend pas en charge la méthode sans authentification",
	KeyErrProxySocksReplyAuth:    "échec de la réponse de méthode d'authentification : %w",
	KeyErrProxySocksReadHeader:   "échec de lecture de l'en-tête de requête : %w",
	KeyErrProxySocksBadVersion:   "version SOCKS invalide : %d",
	KeyErrProxySocksParseAddr:    "échec d'analyse de l'adresse cible : %w",
	KeyErrProxySocksReadPort:     "échec de lecture du port : %w",
	KeyErrProxySocksBadCommand:   "commande non prise en charge : %d",
	KeyErrProxySocksDial:         "échec de connexion à la cible %s : %w",
	KeyErrProxySocksReplySuccess: "échec de la réponse de succès : %w",
	KeyErrProxySocksBadAddrType:  "type d'adresse non pris en charge : %d",

	KeyErrProxyHTTPParseRequest: "échec d'analyse de la requête HTTP : %w",
	KeyErrProxyHTTPConnectDial:  "échec de la connexion CONNECT vers %s : %w",
	KeyErrProxyHTTPConnectReply: "échec de la réponse de succès CONNECT : %w",
	KeyErrProxyHTTPDial:         "échec de connexion à %s : %w",
	KeyErrProxyHTTPForward:      "échec du transfert de la requête vers %s : %w",
	KeyErrProxyHTTPRelayResp:    "échec du relais de la réponse : %w",

	KeyErrProxyUpstreamScheme:        "schéma amont non pris en charge : %q (choisir socks5, http ou ssh)",
	KeyErrProxyUpstreamParse:         "échec d'analyse de l'URL amont : %w",
	KeyErrProxyUpstreamEmptyHost:     "l'URL amont doit inclure un hôte",
	KeyErrProxyUpstreamSocks5:        "échec de création du dialer amont SOCKS5 : %w",
	KeyErrProxyUpstreamHTTPConnect:   "échec du CONNECT HTTP amont vers %s : %w",
	KeyErrProxyUpstreamHTTPStatus:    "le CONNECT HTTP amont vers %s a renvoyé un statut inattendu : %s",
	KeyErrProxyUpstreamSSHNoAuth:     "l'amont ssh nécessite un fichier de clé (-upstream-identity) ou un mot de passe dans l'URL amont",
	KeyErrProxyUpstreamSSHIdentity:   "échec de lecture du fichier de clé SSH %s : %w",
	KeyErrProxyUpstreamSSHParseKey:   "échec d'analyse de la clé privée SSH : %w",
	KeyErrProxyUpstreamSSHKnownHosts: "échec du chargement de SSH known_hosts %s : %w",
	KeyErrProxyUpstreamSSHDial:       "échec de l'établissement de la connexion amont SSH vers %s : %w",
	KeyErrProxyUpstreamSSHChannel:    "échec de l'ouverture du canal SSH vers %s : %w",
	KeyErrProxyUpstreamClosed:        "le dialer amont est fermé",
}
