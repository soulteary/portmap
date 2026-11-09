package proxy

import (
	"log"
	"time"

	"github.com/soulteary/portmap/internal/upstream"
)

// UpstreamConfig describes an outbound transport.
type UpstreamConfig = upstream.UpstreamConfig

// Dialer establishes outbound connections.
type Dialer = upstream.Dialer

const (
	UpstreamSchemeSOCKS5 = upstream.UpstreamSchemeSOCKS5
	UpstreamSchemeHTTP   = upstream.UpstreamSchemeHTTP
	UpstreamSchemeSSH    = upstream.UpstreamSchemeSSH
)

// ParseUpstreamURL parses the upstream endpoint without logging credentials.
func ParseUpstreamURL(raw string) (*UpstreamConfig, error) { return upstream.ParseUpstreamURL(raw) }

// NewUpstreamDialer creates the requested upstream transport.
func NewUpstreamDialer(cfg *UpstreamConfig, timeout, keepAlive time.Duration, logger *log.Logger) (Dialer, error) {
	return upstream.NewUpstreamDialer(cfg, timeout, keepAlive, logger)
}

// NewDirectDialer creates a direct transport that ignores environment proxies.
func NewDirectDialer(timeout, keepAlive time.Duration) Dialer {
	return upstream.NewDirectDialer(timeout, keepAlive)
}
