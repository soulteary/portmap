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
