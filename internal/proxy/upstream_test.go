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
	"bytes"
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
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/soulteary/portmap/internal/i18n"
)

// TestMain 清除进程继承的 SSH_AUTH_SOCK。agent 认证默认开启，若沿用开发机上的
// 真实 agent，SSH 上游用例的认证方式集合就会随环境变化；需要 agent 的用例自行
// 通过 UpstreamConfig.AgentSocket 指向临时 agent。
func TestMain(m *testing.M) {
	_ = os.Unsetenv("SSH_AUTH_SOCK")
	os.Exit(m.Run())
}

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

// generateEncryptedClientKey 返回一个用 passphrase 加密的 PEM 私钥与对应公钥。
func generateEncryptedClientKey(t *testing.T, passphrase string) ([]byte, ssh.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成客户端私钥失败: %v", err)
	}
	block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "", []byte(passphrase))
	if err != nil {
		t.Fatalf("序列化加密私钥失败: %v", err)
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

// TestSSHDialerEncryptedIdentityWithPassphrase 验证：加密私钥 + 正确 passphrase
// 时能成功构造 SSH 上游并拨号可达后端。
func TestSSHDialerEncryptedIdentityWithPassphrase(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	const passphrase = "correct horse battery staple"
	clientPEM, clientPub := generateEncryptedClientKey(t, passphrase)
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:             UpstreamSchemeSSH,
		Addr:               sshAddr,
		Username:           "tester",
		IdentityFile:       identity,
		IdentityPassphrase: passphrase,
		Insecure:           true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("加密私钥 + 正确 passphrase 应成功: %v", err)
	}
	defer closeDialer(t, dialer)

	assertDialerReachesBackend(t, dialer, backendAddr)
}

// TestSSHDialerEncryptedIdentityWrongPassphrase 验证：加密私钥 + 错误 passphrase
// 时返回解析错误。
func TestSSHDialerEncryptedIdentityWrongPassphrase(t *testing.T) {
	clientPEM, clientPub := generateEncryptedClientKey(t, "right-pass")
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	_, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:             UpstreamSchemeSSH,
		Addr:               sshAddr,
		Username:           "tester",
		IdentityFile:       identity,
		IdentityPassphrase: "wrong-pass",
		Insecure:           true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err == nil {
		t.Fatal("期望错误 passphrase 时返回错误")
	}
}

// TestSSHDialerEncryptedIdentityMissingPassphrase 验证：加密私钥但未提供
// passphrase 时返回明确的 "passphrase missing" 错误（i18n 文案），而非通用解析错误。
func TestSSHDialerEncryptedIdentityMissingPassphrase(t *testing.T) {
	clientPEM, clientPub := generateEncryptedClientKey(t, "some-pass")
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	_, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:       UpstreamSchemeSSH,
		Addr:         sshAddr,
		Username:     "tester",
		IdentityFile: identity,
		Insecure:     true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err == nil {
		t.Fatal("期望加密私钥但缺少 passphrase 时返回错误")
	}
	want := i18n.T(i18n.KeyErrProxyUpstreamSSHPassphraseMissing)
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("期望返回 passphrase missing 错误，实际: %v", err)
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

func TestSSHDialerChannelFailureKeepsActiveChannels(t *testing.T) {
	echoLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听 echo 后端失败: %v", err)
	}
	defer func() { _ = echoLn.Close() }()
	go func() {
		for {
			conn, acceptErr := echoLn.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				defer func() { _ = conn.Close() }()
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()
	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:            UpstreamSchemeSSH,
		Addr:              sshAddr,
		Username:          "tester",
		IdentityFile:      writeIdentity(t, clientPEM),
		Insecure:          true,
		KeepaliveInterval: -1,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	active, err := dialer.DialContext(context.Background(), "tcp", echoLn.Addr().String())
	if err != nil {
		t.Fatalf("打开活跃 channel: %v", err)
	}
	defer func() { _ = active.Close() }()

	closedLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("获取关闭端口: %v", err)
	}
	deadAddr := closedLn.Addr().String()
	_ = closedLn.Close()
	if _, err := dialer.DialContext(context.Background(), "tcp", deadAddr); err == nil {
		t.Fatal("不可达目标应返回 channel 错误")
	}

	_ = active.SetDeadline(time.Now().Add(time.Second))
	if _, err := active.Write([]byte("still-alive")); err != nil {
		t.Fatalf("目标 channel 失败后活跃 channel 写入失败: %v", err)
	}
	got := make([]byte, len("still-alive"))
	if _, err := io.ReadFull(active, got); err != nil {
		t.Fatalf("目标 channel 失败后活跃 channel 已被关闭: %v", err)
	}
	if string(got) != "still-alive" {
		t.Fatalf("echo=%q，期望 still-alive", got)
	}
}

func TestSSHDialerInitialHandshakeHonorsContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听停滞 SSH 端点失败: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = conn.Read(make([]byte, 1))
	}()

	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:            UpstreamSchemeSSH,
		Addr:              ln.Addr().String(),
		Username:          "tester",
		Password:          "secret",
		Insecure:          true,
		KeepaliveInterval: -1,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:1"); err == nil {
		t.Fatal("停滞握手应随 context 取消")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("SSH 初始握手未及时响应 context: %s", elapsed)
	}
}

func TestSSHDialerHealthProbeHonorsContext(t *testing.T) {
	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServerWithRequestReplies(t, clientPub, false)
	defer stopSSH()

	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:            UpstreamSchemeSSH,
		Addr:              sshAddr,
		Username:          "tester",
		IdentityFile:      writeIdentity(t, clientPEM),
		Insecure:          true,
		KeepaliveInterval: -1,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := dialer.DialContext(ctx, "tcp", "127.0.0.1:1"); err == nil {
		t.Fatal("停滞健康探针应随 context 取消")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("SSH 健康探针未及时响应 context: %s", elapsed)
	}
}

func TestSSHDialerPassiveRevalidationDiscardsBlackholedClient(t *testing.T) {
	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServerWithRequestReplies(t, clientPub, false)
	defer stopSSH()

	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:            UpstreamSchemeSSH,
		Addr:              sshAddr,
		Username:          "tester",
		IdentityFile:      writeIdentity(t, clientPEM),
		Insecure:          true,
		KeepaliveInterval: -1,
	}, 80*time.Millisecond, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	sd := dialer.(*sshDialer)
	client, err := sd.getClient(context.Background())
	if err != nil {
		t.Fatalf("建立 SSH 客户端失败: %v", err)
	}
	sd.revalidateAfterTimeout(client)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		sd.mu.Lock()
		cached := sd.client
		sd.mu.Unlock()
		if cached == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("被动重校验未丢弃无响应的 SSH 客户端")
}

func TestSSHDialerSupervisorReplacesBlackholedClient(t *testing.T) {
	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServerWithRequestReplies(t, clientPub, false)
	defer stopSSH()

	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:               UpstreamSchemeSSH,
		Addr:                 sshAddr,
		Username:             "tester",
		IdentityFile:         writeIdentity(t, clientPEM),
		Insecure:             true,
		KeepaliveInterval:    10 * time.Millisecond,
		KeepaliveMaxFailures: 3,
	}, 80*time.Millisecond, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	sd := dialer.(*sshDialer)
	initial, err := sd.getClient(context.Background())
	if err != nil {
		t.Fatalf("建立 SSH 客户端失败: %v", err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		sd.mu.Lock()
		current := sd.client
		sd.mu.Unlock()
		if current != nil && current != initial {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("supervisor 未替换无响应的 SSH 客户端")
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

// startTestProxyWithUpstream 启动一个使用给定上游的完整代理服务，返回其地址。
// 与 startTestProxy 一样预建监听器以避免地址发现竞态，但保留 New 的默认空闲
// 超时，使中继阶段真正走共享截止时间刷新路径。
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

// TestSSHUpstreamServesSOCKS5AndConnect 覆盖「代理服务 + SSH 上游」的完整链路。
// SSH 通道原生不支持 SetDeadline，若不为其补齐截止时间语义，共享空闲超时在
// 每次读写前的刷新都会失败，SOCKS5 成功应答与 CONNECT 应答将完全无法写出。
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

// assertHTTPGetOverConn 在已建立的隧道上发起一次最小 HTTP 请求并校验正文。
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

// TestSSHChanConnDeadline 验证 SSH 通道的截止时间模拟：到期即关闭连接使阻塞
// 的读取返回，零值清除定时器，且半关闭能力被显式转发。
func TestSSHChanConnDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	defer func() { _ = ln.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() {
		if c, aerr := ln.Accept(); aerr == nil {
			accepted <- c
		}
	}()
	raw, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("拨号失败: %v", err)
	}
	var server net.Conn
	select {
	case server = <-accepted:
	case <-time.After(time.Second):
		t.Fatal("未接受连接")
	}
	defer func() { _ = server.Close() }()

	conn := &sshChanConn{Conn: raw}
	// 零值不排定任何到期动作，后续读取应保持阻塞直到超时被真正设置。
	if err := conn.SetDeadline(time.Time{}); err != nil {
		t.Fatalf("清除截止时间失败: %v", err)
	}
	// CloseWrite 需转发到底层 TCP 连接，使对端读到 EOF。
	if err := conn.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite 应转发到底层连接: %v", err)
	}
	if _, err := io.ReadAll(server); err != nil {
		t.Fatalf("半关闭后对端读取应正常结束: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatalf("设置读截止时间失败: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, readErr := conn.Read(make([]byte, 1))
		done <- readErr
	}()
	select {
	case readErr := <-done:
		if readErr == nil {
			t.Fatal("到期后读取应返回错误")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("到期后读取未解除阻塞")
	}
	// 连接已被定时器关闭，Close 仍应安全（同时停掉定时器）。
	_ = conn.Close()
}

// ---- 主动保活 / 后台守护重连测试 ----

// closableSSHServer 是可控的内存 SSH 服务端：除处理 direct-tcpip 通道外，
// 还记录每个已接受的底层连接，便于测试从服务端主动断开，触发客户端重连。
type closableSSHServer struct {
	addr string
	mu   sync.Mutex
	conn []net.Conn
	ln   net.Listener
}

// closeConns 关闭当前所有服务端连接，模拟服务端主动断开（或网络中断）。
func (s *closableSSHServer) closeConns() {
	s.mu.Lock()
	conns := s.conn
	s.conn = nil
	s.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

func (s *closableSSHServer) track(c net.Conn) {
	s.mu.Lock()
	s.conn = append(s.conn, c)
	s.mu.Unlock()
}

// startClosableSSHServer 启动一个可从服务端主动断开的内存 SSH 服务端。
func startClosableSSHServer(t *testing.T, authorizedKey ssh.PublicKey) (*closableSSHServer, func()) {
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
	srv := &closableSSHServer{addr: ln.Addr().String(), ln: ln}
	go func() {
		for {
			nConn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			srv.track(nConn)
			go handleSSHConn(nConn, serverCfg, true)
		}
	}()
	return srv, func() { _ = ln.Close() }
}

// superviseGoroutineRunning 报告是否有 goroutine 当前处于 superviseClient 中，
// 用于精确检测守护是否已退出（避免受 http/ssh 客户端等无关 goroutine 干扰）。
func superviseGoroutineRunning() bool {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return strings.Contains(string(buf[:n]), "superviseClient")
}

// waitSupervisorExit 轮询等待守护 goroutine 退出。
func waitSupervisorExit(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if !superviseGoroutineRunning() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestSSHDialerKeepaliveKeepsAlive 验证：启用短间隔主动保活后连接持续可用，
// 保活探测本身不会误杀健康连接。
func TestSSHDialerKeepaliveKeepsAlive(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:            UpstreamSchemeSSH,
		Addr:              sshAddr,
		Username:          "tester",
		IdentityFile:      identity,
		Insecure:          true,
		KeepaliveInterval: 20 * time.Millisecond,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	// 首次拨号触发建连并启动守护。
	assertDialerReachesBackend(t, dialer, backendAddr)
	// 等待若干个保活周期，连接应保持可用。
	time.Sleep(150 * time.Millisecond)
	assertDialerReachesBackend(t, dialer, backendAddr)

	// 客户端应始终是同一个（未发生重连）。
	sd := dialer.(*sshDialer)
	sd.mu.Lock()
	client := sd.client
	sd.mu.Unlock()
	if client == nil {
		t.Fatal("期望保活期间客户端保持存活")
	}
}

// TestSSHDialerCloseTerminatesSupervisor 验证：Close 关闭 done 通道后，
// 守护 goroutine 及时退出，无泄漏。
func TestSSHDialerCloseTerminatesSupervisor(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:            UpstreamSchemeSSH,
		Addr:              sshAddr,
		Username:          "tester",
		IdentityFile:      identity,
		Insecure:          true,
		KeepaliveInterval: 20 * time.Millisecond,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	assertDialerReachesBackend(t, dialer, backendAddr)

	// 建连后守护 goroutine 应在运行。
	if !superviseGoroutineRunning() {
		t.Fatal("期望建连后守护 goroutine 正在运行")
	}

	// 关闭后守护 goroutine 应退出。
	closeDialer(t, dialer)
	if !waitSupervisorExit(2 * time.Second) {
		t.Fatal("Close 后守护 goroutine 未退出（疑似泄漏）")
	}
}

// TestSSHDialerAutoReconnectOnServerDisconnect 验证：服务端主动断开后，
// 守护 goroutine 检测到并在退避后自动重连，DialContext 恢复可用。
func TestSSHDialerAutoReconnectOnServerDisconnect(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	clientPEM, clientPub := generateClientKey(t)
	srv, stopSSH := startClosableSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	d, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:            UpstreamSchemeSSH,
		Addr:              srv.addr,
		Username:          "tester",
		IdentityFile:      identity,
		Insecure:          true,
		KeepaliveInterval: 20 * time.Millisecond,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, d)

	sd := d.(*sshDialer)
	// 使用极短退避，避免测试等待过久。
	sd.minBackoff = 10 * time.Millisecond
	sd.maxBackoff = 50 * time.Millisecond

	assertDialerReachesBackend(t, d, backendAddr)

	// 服务端主动断开当前连接，守护应检测到并后台重连。
	srv.closeConns()

	// 轮询直到重连成功（DialContext 可用）。
	deadline := time.Now().Add(3 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, dialErr := d.DialContext(context.Background(), "tcp", backendAddr)
		if dialErr == nil {
			_ = conn.Close()
			lastErr = nil
			break
		}
		lastErr = dialErr
		time.Sleep(20 * time.Millisecond)
	}
	if lastErr != nil {
		t.Fatalf("服务端断开后未能自动重连: %v", lastErr)
	}
}

// TestSSHDialerKeepaliveDisabled 验证：KeepaliveInterval < 0 时禁用主动保活，
// 不启动守护 goroutine，退回到被动重连行为（DialContext 仍能在断开后重连）。
func TestSSHDialerKeepaliveDisabled(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	clientPEM, clientPub := generateClientKey(t)
	sshAddr, _, stopSSH := startSSHServer(t, clientPub)
	defer stopSSH()

	identity := writeIdentity(t, clientPEM)
	d, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:            UpstreamSchemeSSH,
		Addr:              sshAddr,
		Username:          "tester",
		IdentityFile:      identity,
		Insecure:          true,
		KeepaliveInterval: -1,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, d)

	assertDialerReachesBackend(t, d, backendAddr)

	// 禁用主动保活时不应启动守护 goroutine：丢弃当前客户端后，若无守护，
	// 客户端应保持为 nil（不会被后台自动重连拉起），直到下一次显式拨号。
	sd := d.(*sshDialer)
	sd.mu.Lock()
	current := sd.client
	sd.mu.Unlock()
	if current != nil {
		sd.discard(current)
	}
	time.Sleep(100 * time.Millisecond)
	sd.mu.Lock()
	afterDiscard := sd.client
	sd.mu.Unlock()
	if afterDiscard != nil {
		t.Fatal("禁用保活时不应有守护 goroutine 后台重连")
	}

	// 被动重连仍应工作：下一次拨号自动重连。
	assertDialerReachesBackend(t, d, backendAddr)
}

// ---- ssh-agent 上游测试基础设施 ----

// syncBuffer 是并发安全的日志缓冲：dialer 的后台 goroutine 也会写同一个 logger。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// generateAgentKey 生成一把可加入 agent 的 RSA 私钥及其对应公钥。
func generateAgentKey(t *testing.T) (*rsa.PrivateKey, ssh.PublicKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成 agent 私钥失败: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("构造 agent signer 失败: %v", err)
	}
	return key, signer.PublicKey()
}

// tempAgentSocket 返回一个临时 socket 路径。不用 t.TempDir()，是因为 unix socket
// 路径有长度上限（macOS 约 104 字节），而 t.TempDir() 会把用例名拼进路径。
func tempAgentSocket(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "pmagent")
	if err != nil {
		t.Fatalf("创建 agent 临时目录失败: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.Join(dir, "agent.sock")
}

// startTestAgent 在 socket 上启动一个内存 ssh-agent 并载入 keys。返回的 stop
// 关闭监听器（Go 会一并删除 socket 文件），可用于模拟 agent 重启。
func startTestAgent(t *testing.T, socket string, keys ...*rsa.PrivateKey) func() {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows 上 ssh-agent 走 named pipe，不支持")
	}
	ring := agent.NewKeyring()
	for _, key := range keys {
		if err := ring.Add(agent.AddedKey{PrivateKey: key}); err != nil {
			t.Fatalf("向 agent 添加私钥失败: %v", err)
		}
	}
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("agent 监听失败: %v", err)
	}
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func() { _ = agent.ServeAgent(ring, conn) }()
		}
	}()
	return func() { _ = ln.Close() }
}

// TestAgentKeyringSigners 验证 agentKeyring 能从真实 agent 取到签名者。
func TestAgentKeyringSigners(t *testing.T) {
	key, pub := generateAgentKey(t)
	socket := tempAgentSocket(t)
	stopAgent := startTestAgent(t, socket, key)
	defer stopAgent()

	keyring := newAgentKeyring(&UpstreamConfig{AgentSocket: socket})
	if keyring == nil {
		t.Fatal("agent 可达时应启用 agent 认证")
	}
	defer func() { _ = keyring.Close() }()

	signers, err := keyring.signers()
	if err != nil {
		t.Fatalf("获取 agent 签名者失败: %v", err)
	}
	if len(signers) != 1 {
		t.Fatalf("期望 1 个签名者，实际 %d 个", len(signers))
	}
	if string(signers[0].PublicKey().Marshal()) != string(pub.Marshal()) {
		t.Fatal("签名者公钥与加入 agent 的私钥不符")
	}

	// Close 后连接释放，且后续回调不再把新连接挂回 keyring。
	if err := keyring.Close(); err != nil {
		t.Fatalf("关闭 keyring 失败: %v", err)
	}
	if _, err := keyring.signers(); err == nil {
		t.Fatal("keyring 关闭后不应再返回签名者")
	}
	keyring.mu.Lock()
	leaked := keyring.conn
	keyring.mu.Unlock()
	if leaked != nil {
		t.Fatal("keyring 关闭后不应持有 agent 连接")
	}
}

// TestAgentKeyringRecoversAfterAgentRestart 验证 agent 重启（socket 被替换）后，
// 下一次回调重新连接并取到新 agent 的密钥，而非沿用已失效的旧连接。
func TestAgentKeyringRecoversAfterAgentRestart(t *testing.T) {
	firstKey, firstPub := generateAgentKey(t)
	socket := tempAgentSocket(t)
	stopFirst := startTestAgent(t, socket, firstKey)

	keyring := newAgentKeyring(&UpstreamConfig{AgentSocket: socket})
	if keyring == nil {
		t.Fatal("agent 可达时应启用 agent 认证")
	}
	defer func() { _ = keyring.Close() }()

	signers, err := keyring.signers()
	if err != nil {
		t.Fatalf("获取 agent 签名者失败: %v", err)
	}
	if len(signers) != 1 || string(signers[0].PublicKey().Marshal()) != string(firstPub.Marshal()) {
		t.Fatal("首次应取到第一个 agent 的密钥")
	}

	stopFirst()
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		t.Fatalf("移除旧 socket 失败: %v", err)
	}
	secondKey, secondPub := generateAgentKey(t)
	stopSecond := startTestAgent(t, socket, secondKey)
	defer stopSecond()

	signers, err = keyring.signers()
	if err != nil {
		t.Fatalf("agent 重启后获取签名者失败: %v", err)
	}
	if len(signers) != 1 || string(signers[0].PublicKey().Marshal()) != string(secondPub.Marshal()) {
		t.Fatal("agent 重启后应取到新 agent 的密钥")
	}
}

// TestNewAgentKeyringSkipsUnavailable 验证 agent 不可用时静默跳过（返回 nil），
// 使未使用 agent 的既有配置行为不变。
func TestNewAgentKeyringSkipsUnavailable(t *testing.T) {
	key, _ := generateAgentKey(t)
	socket := tempAgentSocket(t)
	stopAgent := startTestAgent(t, socket, key)
	defer stopAgent()

	cases := []struct {
		name string
		cfg  *UpstreamConfig
	}{
		{"未配置 socket 且 SSH_AUTH_SOCK 为空", &UpstreamConfig{}},
		{"socket 路径不存在", &UpstreamConfig{AgentSocket: filepath.Join(t.TempDir(), "missing.sock")}},
		{"显式关闭 agent", &UpstreamConfig{AgentSocket: socket, DisableAgent: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if keyring := newAgentKeyring(c.cfg); keyring != nil {
				t.Fatalf("期望跳过 agent，实际启用了 socket %q", keyring.socket)
			}
		})
	}
}

// TestNewAgentKeyringUsesEnvSocket 验证未显式配置 socket 时回落到 SSH_AUTH_SOCK。
func TestNewAgentKeyringUsesEnvSocket(t *testing.T) {
	key, _ := generateAgentKey(t)
	socket := tempAgentSocket(t)
	stopAgent := startTestAgent(t, socket, key)
	defer stopAgent()

	t.Setenv("SSH_AUTH_SOCK", socket)
	keyring := newAgentKeyring(&UpstreamConfig{})
	if keyring == nil {
		t.Fatal("SSH_AUTH_SOCK 指向可达 agent 时应启用 agent 认证")
	}
	defer func() { _ = keyring.Close() }()
	if keyring.socket != socket {
		t.Fatalf("socket=%q，期望 %q", keyring.socket, socket)
	}
}

// TestAgentKeyringSignersDialError 验证 socket 不可连时 signers 返回错误，
// 由 x/crypto/ssh 回落到其它认证方式。
func TestAgentKeyringSignersDialError(t *testing.T) {
	keyring := &agentKeyring{socket: filepath.Join(t.TempDir(), "missing.sock")}
	if _, err := keyring.signers(); err == nil {
		t.Fatal("期望连接不存在的 agent socket 时返回错误")
	}
}

// TestSSHDialerAgentAuth 验证仅凭 agent（无私钥文件、无密码）即可完成 SSH 上游
// 认证并拨通后端，且 Close 会释放 agent 连接。
func TestSSHDialerAgentAuth(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	key, pub := generateAgentKey(t)
	socket := tempAgentSocket(t)
	stopAgent := startTestAgent(t, socket, key)
	defer stopAgent()

	sshAddr, _, stopSSH := startSSHServer(t, pub)
	defer stopSSH()

	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:      UpstreamSchemeSSH,
		Addr:        sshAddr,
		Username:    "tester",
		AgentSocket: socket,
		Insecure:    true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("构造 SSH 上游失败: %v", err)
	}
	defer closeDialer(t, dialer)

	assertDialerReachesBackend(t, dialer, backendAddr)

	sd := dialer.(*sshDialer)
	if sd.keyring == nil {
		t.Fatal("期望 dialer 持有 agent keyring")
	}
	closeDialer(t, dialer)
	sd.keyring.mu.Lock()
	conn := sd.keyring.conn
	sd.keyring.mu.Unlock()
	if conn != nil {
		t.Fatal("Close 后 agent 连接应已释放")
	}
}

// TestSSHDialerEncryptedIdentityFallsBackToAgent 验证降级逻辑：加密私钥缺
// passphrase 时，agent 可用即从致命错误降级为告警，并继续用 agent 完成认证。
func TestSSHDialerEncryptedIdentityFallsBackToAgent(t *testing.T) {
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	agentKey, agentPub := generateAgentKey(t)
	socket := tempAgentSocket(t)
	stopAgent := startTestAgent(t, socket, agentKey)
	defer stopAgent()

	// 服务端只认 agent 里的密钥，私钥文件是另一把加密密钥，只有真的跳过它才能登录。
	sshAddr, _, stopSSH := startSSHServer(t, agentPub)
	defer stopSSH()

	clientPEM, _ := generateEncryptedClientKey(t, "some-pass")
	identity := writeIdentity(t, clientPEM)

	logs := &syncBuffer{}
	dialer, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:       UpstreamSchemeSSH,
		Addr:         sshAddr,
		Username:     "tester",
		IdentityFile: identity,
		AgentSocket:  socket,
		Insecure:     true,
	}, 5*time.Second, 30*time.Second, log.New(logs, "", 0))
	if err != nil {
		t.Fatalf("agent 可用时加密私钥缺 passphrase 不应报错: %v", err)
	}
	defer closeDialer(t, dialer)

	want := i18n.T(i18n.KeyLogProxyUpstreamSSHAgentSkipKey, identity)
	if !strings.Contains(logs.String(), want) {
		t.Fatalf("期望记录跳过私钥的告警，实际日志: %q", logs.String())
	}

	assertDialerReachesBackend(t, dialer, backendAddr)
}

// TestSSHDialerEncryptedIdentityAgentDisabled 验证 -upstream-agent=false 时不发生
// 降级：即使 agent 可达，加密私钥缺 passphrase 仍是致命错误。
func TestSSHDialerEncryptedIdentityAgentDisabled(t *testing.T) {
	agentKey, agentPub := generateAgentKey(t)
	socket := tempAgentSocket(t)
	stopAgent := startTestAgent(t, socket, agentKey)
	defer stopAgent()

	sshAddr, _, stopSSH := startSSHServer(t, agentPub)
	defer stopSSH()

	clientPEM, _ := generateEncryptedClientKey(t, "some-pass")
	identity := writeIdentity(t, clientPEM)

	_, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:       UpstreamSchemeSSH,
		Addr:         sshAddr,
		Username:     "tester",
		IdentityFile: identity,
		AgentSocket:  socket,
		DisableAgent: true,
		Insecure:     true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err == nil {
		t.Fatal("关闭 agent 后加密私钥缺 passphrase 应返回错误")
	}
	if want := i18n.T(i18n.KeyErrProxyUpstreamSSHPassphraseMissing); !strings.Contains(err.Error(), want) {
		t.Fatalf("期望返回 passphrase missing 错误，实际: %v", err)
	}
}

// TestSSHDialerAgentDisabledRequiresOtherAuth 验证 DisableAgent 时 agent 不计入
// 认证方式，缺少其它凭据仍报「无可用认证方式」。
func TestSSHDialerAgentDisabledRequiresOtherAuth(t *testing.T) {
	key, pub := generateAgentKey(t)
	socket := tempAgentSocket(t)
	stopAgent := startTestAgent(t, socket, key)
	defer stopAgent()

	sshAddr, _, stopSSH := startSSHServer(t, pub)
	defer stopSSH()

	_, err := NewUpstreamDialer(&UpstreamConfig{
		Scheme:       UpstreamSchemeSSH,
		Addr:         sshAddr,
		Username:     "tester",
		AgentSocket:  socket,
		DisableAgent: true,
		Insecure:     true,
	}, 5*time.Second, 30*time.Second, log.New(io.Discard, "", 0))
	if err == nil {
		t.Fatal("期望关闭 agent 且无其它凭据时返回错误")
	}
	if want := i18n.T(i18n.KeyErrProxyUpstreamSSHNoAuth); !strings.Contains(err.Error(), want) {
		t.Fatalf("期望返回缺少认证方式错误，实际: %v", err)
	}
}

func TestAgentKeyringSignersHonorsTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 上 ssh-agent 走 named pipe，不支持")
	}
	socket := tempAgentSocket(t)
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	t.Cleanup(func() {
		close(stop)
		_ = ln.Close()
	})
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func() {
				<-stop
				_ = conn.Close()
			}()
		}
	}()

	const timeout = 50 * time.Millisecond
	keyring := newAgentKeyringWithTimeout(&UpstreamConfig{AgentSocket: socket}, timeout)
	if keyring == nil {
		t.Fatal("reachable stalled agent should pass the connection probe")
	}
	defer func() { _ = keyring.Close() }()

	start := time.Now()
	if _, err := keyring.signers(); err == nil {
		t.Fatal("stalled agent should fail when its deadline expires")
	}
	if elapsed := time.Since(start); elapsed > 10*timeout {
		t.Fatalf("agent operation ignored timeout: %s", elapsed)
	}
}
