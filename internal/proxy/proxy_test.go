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
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// startTestProxy 启动一个监听随机端口的代理服务，返回其地址与关闭函数。
func startTestProxy(t *testing.T) (string, func()) {
	t.Helper()
	srv := New("127.0.0.1:0")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	srv.listener = ln
	srv.Addr = ln.Addr().String()
	srv.DialTimeout = 5 * time.Second
	srv.dialer = NewDirectDialer(srv.DialTimeout, defaultKeepAlive)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go srv.serveConn(conn)
		}
	}()

	return ln.Addr().String(), func() { _ = ln.Close() }
}

// startBackend 启动一个简单的后端 HTTP 服务。
func startBackend(t *testing.T) (string, func()) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello from backend")
	})
	srv := &http.Server{Handler: mux}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("后端监听失败: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	return ln.Addr().String(), func() { _ = srv.Close() }
}

// socks5Dial 通过 SOCKS5 代理建立到 target 的 CONNECT 连接。
//
// 这里内联实现一个最小的 SOCKS5 客户端（无认证 + CONNECT + 域名/IPv4），
// 以避免引入 golang.org/x/net/proxy 这样的新第三方依赖，
// 保持 portmap 零新增依赖的定位。
func socks5Dial(proxyAddr, target string) (net.Conn, error) {
	host, portStr, err := net.SplitHostPort(target)
	if err != nil {
		return nil, err
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	// 方法协商：VER=5, NMETHODS=1, METHOD=0（无认证）。
	if _, err := conn.Write([]byte{socks5Version, 0x01, socksAuthNone}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if reply[0] != socks5Version || reply[1] != socksAuthNone {
		_ = conn.Close()
		return nil, fmt.Errorf("SOCKS5 方法协商失败: % x", reply)
	}

	// 请求：VER | CMD=CONNECT | RSV | ATYP | ADDR | PORT。
	req := []byte{socks5Version, socksCmdConnect, 0x00}
	if ip := net.ParseIP(host); ip != nil && ip.To4() != nil {
		req = append(req, socksAddrIPv4)
		req = append(req, ip.To4()...)
	} else {
		req = append(req, socksAddrDomain, byte(len(host)))
		req = append(req, host...)
	}
	var portBuf [2]byte
	binary.BigEndian.PutUint16(portBuf[:], uint16(port))
	req = append(req, portBuf[:]...)
	if _, err := conn.Write(req); err != nil {
		_ = conn.Close()
		return nil, err
	}

	// 应答：VER | REP | RSV | ATYP | BND.ADDR | BND.PORT。
	head := make([]byte, 4)
	if _, err := io.ReadFull(conn, head); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if head[1] != socksRepSuccess {
		_ = conn.Close()
		return nil, fmt.Errorf("SOCKS5 CONNECT 失败, REP=%d", head[1])
	}
	// 消费 BND.ADDR + BND.PORT。
	var skip int
	switch head[3] {
	case socksAddrIPv4:
		skip = net.IPv4len + 2
	case socksAddrIPv6:
		skip = net.IPv6len + 2
	case socksAddrDomain:
		n := make([]byte, 1)
		if _, err := io.ReadFull(conn, n); err != nil {
			_ = conn.Close()
			return nil, err
		}
		skip = int(n[0]) + 2
	default:
		_ = conn.Close()
		return nil, fmt.Errorf("未知 ATYP: %d", head[3])
	}
	if skip > 0 {
		if _, err := io.ReadFull(conn, make([]byte, skip)); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	return conn, nil
}

func TestHTTPProxy(t *testing.T) {
	proxyAddr, stopProxy := startTestProxy(t)
	defer stopProxy()
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	proxyURL, _ := url.Parse("http://" + proxyAddr)
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   5 * time.Second,
	}

	resp, err := client.Get("http://" + backendAddr + "/")
	if err != nil {
		t.Fatalf("通过 HTTP 代理请求失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "hello from backend") {
		t.Fatalf("响应内容不符: %q", body)
	}
}

func TestSOCKS5Proxy(t *testing.T) {
	proxyAddr, stopProxy := startTestProxy(t)
	defer stopProxy()
	backendAddr, stopBackend := startBackend(t)
	defer stopBackend()

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, addr string) (net.Conn, error) {
				return socks5Dial(proxyAddr, addr)
			},
		},
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get("http://" + backendAddr + "/")
	if err != nil {
		t.Fatalf("通过 SOCKS5 代理请求失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "hello from backend") {
		t.Fatalf("响应内容不符: %q", body)
	}
}

func TestStripHopByHopHeadersConnectionOptions(t *testing.T) {
	header := http.Header{
		"Connection":       {"keep-alive, X-Remove", "Another-Hop"},
		"X-Remove":         {"secret"},
		"Another-Hop":      {"secret"},
		"Proxy-Connection": {"keep-alive"},
		"X-End-To-End":     {"keep"},
	}

	stripHopByHopHeaders(header)
	for _, key := range []string{"Connection", "X-Remove", "Another-Hop", "Proxy-Connection"} {
		if got := header.Get(key); got != "" {
			t.Errorf("逐跳首部 %s 未删除: %q", key, got)
		}
	}
	if got := header.Get("X-End-To-End"); got != "keep" {
		t.Fatalf("端到端首部被误删: %q", got)
	}
}

func TestHTTPProxySanitizesBothDirectionsAndAddsVia(t *testing.T) {
	proxyAddr, stopProxy := startTestProxy(t)
	defer stopProxy()

	requestHeaders := make(chan http.Header, 1)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听后端失败: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		backendConn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = backendConn.Close() }()
		req, readErr := http.ReadRequest(bufio.NewReader(backendConn))
		if readErr != nil {
			return
		}
		requestHeaders <- req.Header.Clone()
		_, _ = io.WriteString(backendConn,
			"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nVia: 1.0 origin-gateway\r\nConnection: X-Origin-Secret\r\nX-Origin-Secret: must-not-leak\r\nX-End-To-End: kept\r\n\r\nok")
	}()

	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second)
	if err != nil {
		t.Fatalf("连接代理失败: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := fmt.Fprintf(conn,
		"GET http://%s/ HTTP/1.1\r\nHost: %s\r\nConnection: X-Client-Secret\r\nX-Client-Secret: must-not-leak\r\nX-End-To-End: kept\r\nVia: 1.0 client-gateway\r\n\r\n",
		ln.Addr(), ln.Addr()); err != nil {
		t.Fatalf("发送代理请求失败: %v", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("读取代理响应失败: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got := resp.Header.Get("X-Origin-Secret"); got != "" {
		t.Errorf("响应侧 Connection 命名首部泄漏: %q", got)
	}
	if got := resp.Header.Get("X-End-To-End"); got != "kept" {
		t.Errorf("响应端到端首部丢失: %q", got)
	}
	responseVia := strings.Join(resp.Header.Values("Via"), ", ")
	if !strings.Contains(responseVia, "1.0 origin-gateway") || !strings.Contains(responseVia, "1.1 portmap") {
		t.Errorf("响应 Via 不完整: %q", responseVia)
	}

	select {
	case header := <-requestHeaders:
		if got := header.Get("X-Client-Secret"); got != "" {
			t.Errorf("请求侧 Connection 命名首部泄漏: %q", got)
		}
		if got := header.Get("X-End-To-End"); got != "kept" {
			t.Errorf("请求端到端首部丢失: %q", got)
		}
		requestVia := strings.Join(header.Values("Via"), ", ")
		if !strings.Contains(requestVia, "1.0 client-gateway") || !strings.Contains(requestVia, "1.1 portmap") {
			t.Errorf("请求 Via 不完整: %q", requestVia)
		}
	case <-time.After(time.Second):
		t.Fatal("后端未收到请求")
	}
}

func TestHTTPProxyForwardsInformationalResponses(t *testing.T) {
	proxyAddr, stopProxy := startTestProxy(t)
	defer stopProxy()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听后端失败: %v", err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		backendConn, acceptErr := ln.Accept()
		if acceptErr != nil {
			return
		}
		defer func() { _ = backendConn.Close() }()
		if _, readErr := http.ReadRequest(bufio.NewReader(backendConn)); readErr != nil {
			return
		}
		_, _ = io.WriteString(backendConn,
			"HTTP/1.1 103 Early Hints\r\nLink: </style.css>; rel=preload\r\n\r\n"+
				"HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\nok")
	}()

	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second)
	if err != nil {
		t.Fatalf("连接代理失败: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := fmt.Fprintf(conn, "GET http://%s/ HTTP/1.1\r\nHost: %s\r\n\r\n", ln.Addr(), ln.Addr()); err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}

	reader := bufio.NewReader(conn)
	early, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("读取 103 响应失败: %v", err)
	}
	_ = early.Body.Close()
	if early.StatusCode != http.StatusEarlyHints || !strings.Contains(early.Header.Get("Via"), "1.1 portmap") {
		t.Fatalf("103 响应转发不正确: status=%d Via=%q", early.StatusCode, early.Header.Get("Via"))
	}
	if early.Close {
		t.Fatal("信息响应不应要求关闭连接")
	}

	final, err := http.ReadResponse(reader, nil)
	if err != nil {
		t.Fatalf("读取最终响应失败: %v", err)
	}
	defer func() { _ = final.Body.Close() }()
	body, _ := io.ReadAll(final.Body)
	if final.StatusCode != http.StatusOK || string(body) != "ok" || !strings.Contains(final.Header.Get("Via"), "1.1 portmap") {
		t.Fatalf("最终响应转发不正确: status=%d body=%q Via=%q", final.StatusCode, body, final.Header.Get("Via"))
	}
}
