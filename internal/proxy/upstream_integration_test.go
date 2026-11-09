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

package proxy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// Keep proxy regressions independent of the developer's SSH agent.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("SSH_AUTH_SOCK")
	os.Exit(m.Run())
}

func TestSOCKS5UpstreamDialer(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()
	// 复用 portmap 自身的 SOCKS5 服务端作为上游节点。
	upstreamAddr, stopUpstream := startTestProxy(t)
	defer stopUpstream()

	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme: UpstreamSchemeSOCKS5,
		Addr:   upstreamAddr,
	}, 5*time.Second, 30*time.Second, nil)
	if err != nil {
		t.Fatalf("构造 SOCKS5 上游拨号器失败: %v", err)
	}
	assertDialerReachesBackend(t, dialer, backendAddr)
}

func startTestProxyWithUpstream(t *testing.T, cfg *UpstreamConfig) (string, func()) {
	t.Helper()
	srv := New("127.0.0.1:0")
	srv.Logger = log.New(io.Discard, "", 0)
	srv.DialTimeout = 5 * time.Second
	dialer, err := NewUpstreamDialer(cfg, srv.DialTimeout, defaultKeepAlive, srv.Logger)
	if err != nil {
		t.Fatalf("构造上游拨号器失败: %v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		closeDialer(t, dialer)
		t.Fatalf("监听失败: %v", err)
	}
	srv.listener = ln
	srv.Addr = ln.Addr().String()
	srv.dialer = dialer

	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go srv.serveConn(conn)
		}
	}()

	return ln.Addr().String(), func() {
		_ = ln.Close()
		closeDialer(t, dialer)
	}
}

func TestSSHUpstreamServesSOCKS5AndConnect(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	proxyAddr, stopProxy := startTestProxyWithUpstream(t, &UpstreamConfig{
		Scheme:       UpstreamSchemeSSH,
		Addr:         sshAddr,
		Username:     "tester",
		IdentityFile: writeIdentity(t, clientPEM),
		Insecure:     true,
	})
	defer stopProxy()

	t.Run("socks5", func(t *testing.T) {
		conn, err := socks5Dial(proxyAddr, backendAddr)
		if err != nil {
			t.Fatalf("经 SSH 上游的 SOCKS5 CONNECT 失败: %v", err)
		}
		defer func() { _ = conn.Close() }()
		assertHTTPGetOverConn(t, conn, backendAddr)
	})

	t.Run("http", func(t *testing.T) {
		client := &http.Client{
			Transport: &http.Transport{Proxy: func(*http.Request) (*url.URL, error) {
				return url.Parse("http://" + proxyAddr)
			}},
			Timeout: 5 * time.Second,
		}
		resp, err := client.Get("http://" + backendAddr + "/")
		if err != nil {
			t.Fatalf("经 SSH 上游的 HTTP 代理请求失败: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "hello from backend") {
			t.Fatalf("响应内容不符: %q", body)
		}
	})
}

func generateHostKey(t *testing.T) ssh.Signer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成 host key 失败: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("构造 host signer 失败: %v", err)
	}
	return signer
}

func generateClientKey(t *testing.T) ([]byte, ssh.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成客户端私钥失败: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(key, "")
	if err != nil {
		t.Fatalf("序列化私钥失败: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("构造客户端 signer 失败: %v", err)
	}
	return pem.EncodeToMemory(block), signer.PublicKey()
}

func startSSHServer(t *testing.T, authorizedKey ssh.PublicKey) (string, ssh.PublicKey, func()) {
	return startSSHServerWithRequestReplies(t, authorizedKey, true)
}

func startSSHServerWithRequestReplies(t *testing.T, authorizedKey ssh.PublicKey, replyRequests bool) (string, ssh.PublicKey, func()) {
	t.Helper()
	hostSigner := generateHostKey(t)
	serverCfg := &ssh.ServerConfig{
		PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if string(key.Marshal()) == string(authorizedKey.Marshal()) {
				return &ssh.Permissions{}, nil
			}
			return nil, io.EOF
		},
	}
	serverCfg.AddHostKey(hostSigner)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("SSH 监听失败: %v", err)
	}
	go func() {
		for {
			nConn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go handleSSHConn(nConn, serverCfg, replyRequests)
		}
	}()
	return ln.Addr().String(), hostSigner.PublicKey(), func() { _ = ln.Close() }
}

func handleSSHConn(nConn net.Conn, cfg *ssh.ServerConfig, replyRequests bool) {
	sshConn, chans, reqs, err := ssh.NewServerConn(nConn, cfg)
	if err != nil {
		_ = nConn.Close()
		return
	}
	if replyRequests {
		go ssh.DiscardRequests(reqs)
	}
	go func() {
		_ = sshConn.Wait()
	}()
	for newChan := range chans {
		if newChan.ChannelType() != "direct-tcpip" {
			_ = newChan.Reject(ssh.UnknownChannelType, "only direct-tcpip supported")
			continue
		}
		go handleDirectTCPIP(newChan)
	}
}

type directTCPIPPayload struct {
	HostToConnect  string
	PortToConnect  uint32
	OriginatorAddr string
	OriginatorPort uint32
}

func handleDirectTCPIP(newChan ssh.NewChannel) {
	var payload directTCPIPPayload
	if err := ssh.Unmarshal(newChan.ExtraData(), &payload); err != nil {
		_ = newChan.Reject(ssh.ConnectionFailed, "bad payload")
		return
	}
	target := net.JoinHostPort(payload.HostToConnect, itoa(payload.PortToConnect))
	remote, err := net.Dial("tcp", target)
	if err != nil {
		_ = newChan.Reject(ssh.ConnectionFailed, err.Error())
		return
	}
	ch, reqs, err := newChan.Accept()
	if err != nil {
		_ = remote.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	go func() { _, _ = io.Copy(remote, ch); _ = remote.Close() }()
	go func() { _, _ = io.Copy(ch, remote); _ = ch.Close() }()
}

func itoa(v uint32) string {
	var buf [10]byte
	i := len(buf)
	if v == 0 {
		return "0"
	}
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func writeIdentity(t *testing.T, pem []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "id_rsa")
	if err := os.WriteFile(path, pem, 0o600); err != nil {
		t.Fatalf("写入私钥失败: %v", err)
	}
	return path
}

func closeDialer(t *testing.T, dialer Dialer) {
	t.Helper()
	if closer, ok := dialer.(io.Closer); ok {
		_ = closer.Close()
	}
}

func assertHTTPGetOverConn(t *testing.T, conn net.Conn, host string) {
	t.Helper()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: "+host+"\r\nConnection: close\r\n\r\n"); err != nil {
		t.Fatalf("隧道内写请求失败: %v", err)
	}
	body, err := io.ReadAll(conn)
	if err != nil {
		t.Fatalf("隧道内读响应失败: %v", err)
	}
	if !strings.Contains(string(body), "hello from backend") {
		t.Fatalf("响应内容不符: %q", body)
	}
}

func assertDialerReachesBackend(t *testing.T, dialer Dialer, backendAddr string) {
	t.Helper()
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp", addr)
			},
		},
		Timeout: 5 * time.Second,
	}
	resp, err := client.Get("http://" + backendAddr + "/")
	if err != nil {
		t.Fatalf("通过上游请求后端失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "hello from backend") {
		t.Fatalf("响应内容不符: %q", body)
	}
}
