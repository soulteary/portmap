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
	"errors"
	"net"
	"syscall"
	"testing"
	"time"
)

// TestHasPort 覆盖 hasPort 各分支：空串、带端口、纯主机、IPv6 字面量。
func TestHasPort(t *testing.T) {
	cases := []struct {
		host string
		want bool
	}{
		{"", false},
		{"example.com", false},
		{"example.com:80", true},
		{"127.0.0.1:443", true},
		{"127.0.0.1", false},
		{"[::1]:80", true},
		{"[2001:db8::1]", false},
		{"::1", true}, // 无括号的裸 IPv6 会被判定为带端口（保留既有行为）。
	}
	for _, c := range cases {
		if got := hasPort(c.host); got != c.want {
			t.Errorf("hasPort(%q)=%v，期望 %v", c.host, got, c.want)
		}
	}
}

// TestContainsByte 覆盖 containsByte 命中与未命中两种情况。
func TestContainsByte(t *testing.T) {
	if !containsByte([]byte{0x00, 0x02}, socksAuthNone) {
		t.Error("应命中无认证方法字节")
	}
	if containsByte([]byte{0x02, 0x03}, socksAuthNone) {
		t.Error("不含 0x00 时不应命中")
	}
	if containsByte(nil, socksAuthNone) {
		t.Error("空方法列表不应命中")
	}
}

// TestReadSOCKSAddr 覆盖 IPv4 / IPv6 / 域名 / 未知类型 四条路径。
func TestReadSOCKSAddr(t *testing.T) {
	t.Run("IPv4", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewReader([]byte{127, 0, 0, 1}))
		host, err := readSOCKSAddr(r, socksAddrIPv4)
		if err != nil || host != "127.0.0.1" {
			t.Fatalf("IPv4 解析=%q,%v", host, err)
		}
	})
	t.Run("IPv6", func(t *testing.T) {
		ip := net.ParseIP("2001:db8::1").To16()
		r := bufio.NewReader(bytes.NewReader(ip))
		host, err := readSOCKSAddr(r, socksAddrIPv6)
		if err != nil || net.ParseIP(host) == nil {
			t.Fatalf("IPv6 解析=%q,%v", host, err)
		}
	})
	t.Run("域名", func(t *testing.T) {
		name := "example.com"
		buf := append([]byte{byte(len(name))}, name...)
		r := bufio.NewReader(bytes.NewReader(buf))
		host, err := readSOCKSAddr(r, socksAddrDomain)
		if err != nil || host != name {
			t.Fatalf("域名解析=%q,%v", host, err)
		}
	})
	t.Run("未知地址类型", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewReader([]byte{0x01}))
		if _, err := readSOCKSAddr(r, 0x09); err == nil {
			t.Fatal("未知 ATYP 应返回错误")
		}
	})
	t.Run("IPv4 数据不足", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewReader([]byte{127, 0}))
		if _, err := readSOCKSAddr(r, socksAddrIPv4); err == nil {
			t.Fatal("数据不足应返回错误")
		}
	})
	t.Run("域名长度字节缺失", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewReader(nil))
		if _, err := readSOCKSAddr(r, socksAddrDomain); err == nil {
			t.Fatal("缺少长度字节应返回错误")
		}
	})
	t.Run("域名内容不足", func(t *testing.T) {
		r := bufio.NewReader(bytes.NewReader([]byte{0x05, 'a'}))
		if _, err := readSOCKSAddr(r, socksAddrDomain); err == nil {
			t.Fatal("域名内容不足应返回错误")
		}
	})
}

// timeoutError 是实现 net.Error 且 Timeout() 为 true 的错误。
type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// TestSocksReplyForDialError 覆盖超时、连接拒绝、通用失败三种映射。
func TestSocksReplyForDialError(t *testing.T) {
	if got := socksReplyForDialError(timeoutError{}); got != socksRepHostUnreach {
		t.Errorf("超时应映射 HostUnreach，实际 %d", got)
	}
	refused := &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	if got := socksReplyForDialError(refused); got != socksRepConnRefused {
		t.Errorf("连接拒绝应映射 ConnRefused，实际 %d", got)
	}
	if got := socksReplyForDialError(errors.New("boom")); got != socksRepGeneralFailure {
		t.Errorf("普通错误应映射 GeneralFailure，实际 %d", got)
	}
}

// TestIsLoopbackListener 覆盖回环与非回环判定。
func TestIsLoopbackListener(t *testing.T) {
	if !isLoopbackListener(&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1080}) {
		t.Error("127.0.0.1 应为回环")
	}
	if isLoopbackListener(&net.TCPAddr{IP: net.IPv4(0, 0, 0, 0), Port: 1080}) {
		t.Error("0.0.0.0 不应为回环")
	}
	if isLoopbackListener(&net.UnixAddr{Name: "/tmp/x"}) {
		t.Error("非 TCP 地址不应为回环")
	}
}

// TestIsLocalIP 验证回环 IP 被识别为本地。
func TestIsLocalIP(t *testing.T) {
	if !isLocalIP(net.IPv4(127, 0, 0, 1)) {
		t.Error("127.0.0.1 应为本地 IP")
	}
	// 一个几乎不可能属于本机的公网 IP。
	if isLocalIP(net.IPv4(203, 0, 113, 1)) {
		t.Error("203.0.113.1 不应为本地 IP")
	}
}

// TestSendSOCKSReply 验证应答报文结构正确（VER/REP/RSV/ATYP + 零地址）。
func TestSendSOCKSReply(t *testing.T) {
	server, client := net.Pipe()
	defer func() { _ = client.Close() }()

	got := make([]byte, 10)
	done := make(chan error, 1)
	go func() {
		_, err := readFull(client, got)
		done <- err
	}()

	s := New("127.0.0.1:0")
	if err := s.sendSOCKSReply(server, socksRepSuccess); err != nil {
		t.Fatalf("sendSOCKSReply: %v", err)
	}
	_ = server.Close()
	if err := <-done; err != nil {
		t.Fatalf("读取应答失败: %v", err)
	}
	want := []byte{socks5Version, socksRepSuccess, 0x00, socksAddrIPv4, 0, 0, 0, 0, 0, 0}
	if !bytes.Equal(got, want) {
		t.Fatalf("应答=% x，期望 % x", got, want)
	}
}

// TestSOCKS5NoAcceptableAuth 验证客户端不提供“无认证”方法时服务端拒绝。
func TestSOCKS5NoAcceptableAuth(t *testing.T) {
	proxyAddr, stop := startTestProxy(t)
	defer stop()

	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second)
	if err != nil {
		t.Fatalf("连接代理失败: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// VER=5, NMETHODS=1, METHOD=0x02（用户名/密码，本服务不支持）。
	if _, err := conn.Write([]byte{socks5Version, 0x01, 0x02}); err != nil {
		t.Fatalf("发送方法协商失败: %v", err)
	}
	reply := make([]byte, 2)
	if _, err := readFull(conn, reply); err != nil {
		t.Fatalf("读取应答失败: %v", err)
	}
	if reply[0] != socks5Version || reply[1] != socksAuthNoAcceptable {
		t.Fatalf("应答=% x，期望无可接受方法", reply)
	}
}

// TestSOCKS5UnsupportedCommand 验证非 CONNECT 命令返回“命令不支持”应答。
func TestSOCKS5UnsupportedCommand(t *testing.T) {
	proxyAddr, stop := startTestProxy(t)
	defer stop()

	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second)
	if err != nil {
		t.Fatalf("连接代理失败: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte{socks5Version, 0x01, socksAuthNone}); err != nil {
		t.Fatalf("方法协商失败: %v", err)
	}
	if _, err := readFull(conn, make([]byte, 2)); err != nil {
		t.Fatalf("读取协商应答失败: %v", err)
	}

	// 请求：VER | CMD=0x02(BIND, 不支持) | RSV | ATYP=IPv4 | 1.2.3.4 | 端口 80。
	req := []byte{socks5Version, 0x02, 0x00, socksAddrIPv4, 1, 2, 3, 4, 0x00, 0x50}
	if _, err := conn.Write(req); err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}
	head := make([]byte, 10)
	if _, err := readFull(conn, head); err != nil {
		t.Fatalf("读取应答失败: %v", err)
	}
	if head[1] != socksRepCmdNotSupported {
		t.Fatalf("应答码=%d，期望命令不支持(%d)", head[1], socksRepCmdNotSupported)
	}
}

// TestSOCKS5BadRequestVersion 验证请求头 VER 非 0x05 时握手失败（覆盖
// handleSOCKS5WithReader 的版本校验分支）。
func TestSOCKS5BadRequestVersion(t *testing.T) {
	proxyAddr, stop := startTestProxy(t)
	defer stop()

	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second)
	if err != nil {
		t.Fatalf("连接代理失败: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte{socks5Version, 0x01, socksAuthNone}); err != nil {
		t.Fatalf("方法协商失败: %v", err)
	}
	if _, err := readFull(conn, make([]byte, 2)); err != nil {
		t.Fatalf("读取协商应答失败: %v", err)
	}

	// 请求头 VER=0x04（非法），服务端应在校验版本时中止握手并关闭连接。
	req := []byte{0x04, socksCmdConnect, 0x00, socksAddrIPv4, 1, 2, 3, 4, 0x00, 0x50}
	if _, err := conn.Write(req); err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}
	// 非法版本不产生应答；连接被关闭后读取应返回 EOF/错误。
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Read(make([]byte, 1)); err == nil {
		t.Fatal("非法请求版本时连接应被关闭")
	}
}

// TestSOCKS5UnsupportedAddrType 验证不支持的地址类型返回“地址类型不支持”应答
// （覆盖 readSOCKSAddr default 分支与 socksRepAddrNotSupported 回复）。
func TestSOCKS5UnsupportedAddrType(t *testing.T) {
	proxyAddr, stop := startTestProxy(t)
	defer stop()

	conn, err := net.DialTimeout("tcp", proxyAddr, time.Second)
	if err != nil {
		t.Fatalf("连接代理失败: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.Write([]byte{socks5Version, 0x01, socksAuthNone}); err != nil {
		t.Fatalf("方法协商失败: %v", err)
	}
	if _, err := readFull(conn, make([]byte, 2)); err != nil {
		t.Fatalf("读取协商应答失败: %v", err)
	}

	// ATYP=0x09（未知地址类型），后续负载无关紧要。
	req := []byte{socks5Version, socksCmdConnect, 0x00, 0x09, 0, 0}
	if _, err := conn.Write(req); err != nil {
		t.Fatalf("发送请求失败: %v", err)
	}
	head := make([]byte, 10)
	if _, err := readFull(conn, head); err != nil {
		t.Fatalf("读取应答失败: %v", err)
	}
	if head[1] != socksRepAddrNotSupported {
		t.Fatalf("应答码=%d，期望地址类型不支持(%d)", head[1], socksRepAddrNotSupported)
	}
}

// TestServerListenAndServeUpstreamStartFailure 验证配置了无法构造的上游时，
// ListenAndServe 在监听前返回错误（覆盖上游拨号器构造失败路径）。
func TestServerListenAndServeUpstreamStartFailure(t *testing.T) {
	srv := New("127.0.0.1:0")
	// SSH 上游但既无私钥也无密码：newSSHDialer 会因“无认证方式”失败。
	srv.Upstream = &UpstreamConfig{Scheme: UpstreamSchemeSSH, Addr: "127.0.0.1:22", Insecure: true}
	if err := srv.ListenAndServe(); err == nil {
		t.Fatal("无法构造上游拨号器时 ListenAndServe 应返回错误")
	}
}

// TestServerAlreadyServing 验证同一 Server 重复 ListenAndServe 返回 already serving。
func TestServerAlreadyServing(t *testing.T) {
	srv := New("127.0.0.1:0")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	srv.listener = ln
	srv.dialer = NewDirectDialer(time.Second, defaultKeepAlive)
	t.Cleanup(func() { _ = ln.Close() })

	if err := srv.ListenAndServe(); !errors.Is(err, errServerAlreadyServing) {
		t.Fatalf("重复服务应返回 already serving，实际 %v", err)
	}
}
func readFull(c net.Conn, buf []byte) (int, error) {
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	total := 0
	for total < len(buf) {
		n, err := c.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
