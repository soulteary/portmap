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
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestParseUpstreamURL(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantErr  bool
		scheme   string
		addr     string
		username string
		password string
	}{
		{name: "socks5 with auth", raw: "socks5://user:pass@127.0.0.1:1080", scheme: "socks5", addr: "127.0.0.1:1080", username: "user", password: "pass"},
		{name: "http default port", raw: "http://example.com", scheme: "http", addr: "example.com:3128"},
		{name: "ssh default port", raw: "ssh://root@example.com", scheme: "ssh", addr: "example.com:22", username: "root"},
		{name: "socks5 default port", raw: "socks5://host", scheme: "socks5", addr: "host:1080"},
		{name: "uppercase scheme", raw: "SOCKS5://host:9050", scheme: "socks5", addr: "host:9050"},
		{name: "unknown scheme", raw: "ftp://host:21", wantErr: true},
		{name: "missing host", raw: "socks5://", wantErr: true},
		{name: "garbage", raw: "://bad url ::", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseUpstreamURL(tc.raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望解析出错，实际成功: %+v", cfg)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if cfg.Scheme != tc.scheme || cfg.Addr != tc.addr {
				t.Fatalf("scheme/addr 不符: got %s/%s want %s/%s", cfg.Scheme, cfg.Addr, tc.scheme, tc.addr)
			}
			if cfg.Username != tc.username || cfg.Password != tc.password {
				t.Fatalf("凭据不符: got %q/%q want %q/%q", cfg.Username, cfg.Password, tc.username, tc.password)
			}
		})
	}
}

func TestNewUpstreamDialerUnsupportedScheme(t *testing.T) {
	_, err := NewUpstreamDialer(&UpstreamConfig{Scheme: "ftp", Addr: "host:21"}, time.Second, time.Second, nil)
	if err == nil {
		t.Fatal("期望不支持的 scheme 返回错误")
	}
}

// startCONNECTServer 启动一个最小的 HTTP CONNECT 上游代理，隧道建立后把两端
// 双向拷贝。requireAuth 非空时校验 Proxy-Authorization。
func startCONNECTServer(t *testing.T, requireAuth string) (string, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				reader := bufio.NewReader(conn)
				req, readErr := http.ReadRequest(reader)
				if readErr != nil {
					_ = conn.Close()
					return
				}
				if req.Method != http.MethodConnect {
					_, _ = io.WriteString(conn, "HTTP/1.1 405 Method Not Allowed\r\n\r\n")
					_ = conn.Close()
					return
				}
				if requireAuth != "" && req.Header.Get("Proxy-Authorization") != requireAuth {
					_, _ = io.WriteString(conn, "HTTP/1.1 407 Proxy Authentication Required\r\n\r\n")
					_ = conn.Close()
					return
				}
				target, dialErr := net.Dial("tcp", req.Host)
				if dialErr != nil {
					_, _ = io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\n")
					_ = conn.Close()
					return
				}
				_, _ = io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n")
				go func() { _, _ = io.Copy(target, reader); _ = target.Close() }()
				_, _ = io.Copy(conn, target)
				_ = conn.Close()
			}()
		}
	}()
	return ln.Addr().String(), func() { _ = ln.Close() }
}

func TestHTTPConnectDialer(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()
	proxyAddr, stopProxy := startCONNECTServer(t, "")
	defer stopProxy()

	dialer, err := NewUpstreamDialer(&UpstreamConfig{Scheme: UpstreamSchemeHTTP, Addr: proxyAddr}, 5*time.Second, 30*time.Second, nil)
	if err != nil {
		t.Fatalf("构造上游拨号器失败: %v", err)
	}
	assertDialerReachesBackend(t, dialer, backendAddr)
}

func TestHTTPConnectDialerWithAuth(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()
	expected := "Basic " + basicAuth("alice", "secret")
	proxyAddr, stopProxy := startCONNECTServer(t, expected)
	defer stopProxy()

	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:   UpstreamSchemeHTTP,
		Addr:     proxyAddr,
		Username: "alice",
		Password: "secret",
	}, 5*time.Second, 30*time.Second, nil)
	if err != nil {
		t.Fatalf("构造上游拨号器失败: %v", err)
	}
	assertDialerReachesBackend(t, dialer, backendAddr)

	// 凭据错误时应被上游拒绝。
	badDialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:   UpstreamSchemeHTTP,
		Addr:     proxyAddr,
		Username: "alice",
		Password: "wrong",
	}, 5*time.Second, 30*time.Second, nil)
	if err != nil {
		t.Fatalf("构造上游拨号器失败: %v", err)
	}
	if _, err := badDialer.DialContext(context.Background(), "tcp", backendAddr); err == nil {
		t.Fatal("期望凭据错误时 CONNECT 失败")
	}
}

func TestHTTPConnectDialerBadGateway(t *testing.T) {
	proxyAddr, stopProxy := startCONNECTServer(t, "")
	defer stopProxy()
	dialer, err := NewUpstreamDialer(&UpstreamConfig{Scheme: UpstreamSchemeHTTP, Addr: proxyAddr}, 2*time.Second, 30*time.Second, nil)
	if err != nil {
		t.Fatalf("构造上游拨号器失败: %v", err)
	}
	// 目标为一个已关闭端口，上游返回 502。
	closedLn, _ := net.Listen("tcp", "127.0.0.1:0")
	deadAddr := closedLn.Addr().String()
	_ = closedLn.Close()
	if _, err := dialer.DialContext(context.Background(), "tcp", deadAddr); err == nil {
		t.Fatal("期望上游 502 时返回错误")
	}
}

// assertDialerReachesBackend 通过 dialer 拨号 backend 并发起一个最小 HTTP 请求，
// 校验响应正文。
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

// ---- SSH 上游测试基础设施 ----

// generateHostKey 生成一个测试用 RSA host key signer。
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

// generateClientKey 返回 PEM 编码的私钥与对应的 ssh.PublicKey。
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

// startSSHServer 启动一个内存 SSH 服务端：接受一个已授权公钥的会话，并处理
// direct-tcpip 通道，把通道数据转发到通道请求指定的目标地址。
// 返回监听地址与 host 公钥。
func startSSHServer(t *testing.T, authorizedKey ssh.PublicKey) (string, ssh.PublicKey, func()) {
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
			go handleSSHConn(nConn, serverCfg)
		}
	}()
	return ln.Addr().String(), hostSigner.PublicKey(), func() { _ = ln.Close() }
}

func handleSSHConn(nConn net.Conn, cfg *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(nConn, cfg)
	if err != nil {
		_ = nConn.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
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

// directTCPIPPayload 是 RFC 4254 §7.2 direct-tcpip 通道打开请求的载荷。
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

// writeKnownHosts 生成一个 known_hosts 文件，授权 addr 使用 hostKey。
func writeKnownHosts(t *testing.T, addr string, hostKey ssh.PublicKey) string {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("拆分地址失败: %v", err)
	}
	// 非标准端口需用 [host]:port 形式。
	hostPattern := host
	if port != "22" {
		hostPattern = "[" + host + "]:" + port
	}
	line := hostPattern + " " + string(ssh.MarshalAuthorizedKey(hostKey))
	dir := t.TempDir()
	path := filepath.Join(dir, "known_hosts")
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatalf("写入 known_hosts 失败: %v", err)
	}
	return path
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

func TestSSHDialerInsecureHostKey(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:       UpstreamSchemeSSH,
		Addr:         sshAddr,
		Username:     "tester",
		IdentityFile: identity,
		Insecure:     true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	assertDialerReachesBackend(t, dialer, backendAddr)
}

func TestSSHDialerKnownHostsVerification(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	clientPEM, clientPub := generateClientKey(t)
	sshAddr, hostKey, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	knownHosts := writeKnownHosts(t, sshAddr, hostKey)

	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:         UpstreamSchemeSSH,
		Addr:           sshAddr,
		Username:       "tester",
		IdentityFile:   identity,
		KnownHostsFile: knownHosts,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	assertDialerReachesBackend(t, dialer, backendAddr)
}

func TestSSHDialerKnownHostsMismatch(t *testing.T) {
	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	// 用一个不相关的 host key 写入 known_hosts，触发校验失败。
	wrongHostKey := generateHostKey(t).PublicKey()
	identity := writeIdentity(t, clientPEM)
	knownHosts := writeKnownHosts(t, sshAddr, wrongHostKey)

	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:         UpstreamSchemeSSH,
		Addr:           sshAddr,
		Username:       "tester",
		IdentityFile:   identity,
		KnownHostsFile: knownHosts,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	if _, err := dialer.DialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("期望 host key 不匹配时拨号失败")
	}
}

func TestSSHDialerRequiresAuth(t *testing.T) {
	_, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:   UpstreamSchemeSSH,
		Addr:     "127.0.0.1:22",
		Username: "tester",
		Insecure: true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err == nil {
		t.Fatal("期望缺少认证方式时返回错误")
	}
}

func TestSSHDialerBadIdentityFile(t *testing.T) {
	_, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:       UpstreamSchemeSSH,
		Addr:         "127.0.0.1:22",
		Username:     "tester",
		IdentityFile: filepath.Join(t.TempDir(), "does-not-exist"),
		Insecure:     true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err == nil {
		t.Fatal("期望私钥文件缺失时返回错误")
	}
}

func TestSSHDialerReconnectAfterClose(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:       UpstreamSchemeSSH,
		Addr:         sshAddr,
		Username:     "tester",
		IdentityFile: identity,
		Insecure:     true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	assertDialerReachesBackend(t, dialer, backendAddr)

	// 强制丢弃当前 ssh 客户端，模拟断开；下一次拨号应触发重连。
	sd := dialer.(*sshDialer)
	sd.mu.Lock()
	current := sd.client
	sd.mu.Unlock()
	if current != nil {
		sd.discard(current)
	}
	assertDialerReachesBackend(t, dialer, backendAddr)
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

func closeDialer(t *testing.T, dialer Dialer) {
	t.Helper()
	if closer, ok := dialer.(io.Closer); ok {
		_ = closer.Close()
	}
}
