package forward

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/soulteary/portmap/internal/upstream"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHServer authenticates a generated client key and serves direct-tcpip channels.
type SSHServer struct {
	Addr        string
	Identity    string
	KnownHosts  string
	Connections atomic.Int64
	listener    net.Listener
	mu          sync.Mutex
	conns       map[net.Conn]struct{}
	wg          sync.WaitGroup
	cancel      context.CancelFunc
}

// StartSSH starts a loopback-only server. resolve may map a remote address to a
// local fixture, reject it, or block until ctx is canceled.
func StartSSH(t *testing.T, resolve func(context.Context, string) (string, error)) *SSHServer {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	_, hostKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, presented ssh.PublicKey) (*ssh.Permissions, error) {
		if string(presented.Marshal()) != string(signer.PublicKey().Marshal()) {
			return nil, fmt.Errorf("unauthorized test key")
		}
		return nil, nil
	}}
	cfg.AddHostKey(hostSigner)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &SSHServer{Addr: ln.Addr().String(), listener: ln, conns: make(map[net.Conn]struct{}), cancel: cancel}
	t.Cleanup(s.Close)
	dir := t.TempDir()
	s.Identity = filepath.Join(dir, "identity")
	s.KnownHosts = filepath.Join(dir, "known_hosts")
	block, err := ssh.MarshalPrivateKey(key, "generated test key")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Identity, pem.EncodeToMemory(block), 0600); err != nil {
		t.Fatal(err)
	}
	line := knownhosts.Line([]string{knownhosts.Normalize(s.Addr)}, hostSigner.PublicKey()) + "\n"
	if err := os.WriteFile(s.KnownHosts, []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		for {
			raw, err := ln.Accept()
			if err != nil {
				return
			}
			s.mu.Lock()
			s.conns[raw] = struct{}{}
			s.mu.Unlock()
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				defer func() { _ = raw.Close(); s.mu.Lock(); delete(s.conns, raw); s.mu.Unlock() }()
				stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
				defer stop()
				conn, channels, requests, err := ssh.NewServerConn(raw, cfg)
				if err != nil {
					return
				}
				defer func() { _ = conn.Close() }()
				s.Connections.Add(1)
				go ssh.DiscardRequests(requests)
				for channel := range channels {
					s.wg.Add(1)
					go func() { defer s.wg.Done(); s.serveChannel(ctx, channel, resolve) }()
				}
			}()
		}
	}()
	return s
}

func (s *SSHServer) serveChannel(ctx context.Context, channel ssh.NewChannel, resolve func(context.Context, string) (string, error)) {
	if channel.ChannelType() != "direct-tcpip" {
		_ = channel.Reject(ssh.UnknownChannelType, "unsupported")
		return
	}
	var payload struct {
		Host       string
		Port       uint32
		Origin     string
		OriginPort uint32
	}
	if err := ssh.Unmarshal(channel.ExtraData(), &payload); err != nil {
		_ = channel.Reject(ssh.ConnectionFailed, "invalid payload")
		return
	}
	target := net.JoinHostPort(payload.Host, strconv.Itoa(int(payload.Port)))
	var err error
	if resolve != nil {
		target, err = resolve(ctx, target)
	}
	if err != nil {
		_ = channel.Reject(ssh.ConnectionFailed, "test target unavailable")
		return
	}
	remote, err := (&net.Dialer{}).DialContext(ctx, "tcp", target)
	if err != nil {
		_ = channel.Reject(ssh.ConnectionFailed, "test target unavailable")
		return
	}
	defer func() { _ = remote.Close() }()
	ch, requests, err := channel.Accept()
	if err != nil {
		return
	}
	defer func() { _ = ch.Close() }()
	stop := context.AfterFunc(ctx, func() { _ = remote.Close(); _ = ch.Close() })
	defer stop()
	go ssh.DiscardRequests(requests)
	done := make(chan struct{})
	go func() { defer close(done); _, _ = io.Copy(remote, ch); _ = remote.(*net.TCPConn).CloseWrite() }()
	_, _ = io.Copy(ch, remote)
	_ = ch.CloseWrite()
	// A transport drop must also release a backend blocked in a read.
	_ = remote.Close()
	<-done
}

// Config returns independent settings that never use the developer's agent.
func (s *SSHServer) Config() *upstream.UpstreamConfig {
	return &upstream.UpstreamConfig{Scheme: upstream.UpstreamSchemeSSH, Addr: s.Addr, Username: "test", IdentityFile: s.Identity, KnownHostsFile: s.KnownHosts, DisableAgent: true, KeepaliveInterval: -1}
}

// DropConnections simulates transport loss while retaining the listener.
func (s *SSHServer) DropConnections() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns {
		_ = conn.Close()
	}
}

// Close releases the listener, transports, and pending channels.
func (s *SSHServer) Close() {
	s.cancel()
	_ = s.listener.Close()
	s.DropConnections()
	s.wg.Wait()
}
