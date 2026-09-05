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
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/netutil"
)

// SOCKS5 协议常量，参见 RFC 1928。
const (
	socks5Version = 0x05

	// 认证方法
	socksAuthNone         = 0x00
	socksAuthNoAcceptable = 0xFF

	// 命令
	socksCmdConnect = 0x01

	// 地址类型
	socksAddrIPv4   = 0x01
	socksAddrDomain = 0x03
	socksAddrIPv6   = 0x04

	// 应答状态
	socksRepSuccess          = 0x00
	socksRepGeneralFailure   = 0x01
	socksRepHostUnreach      = 0x04
	socksRepConnRefused      = 0x05
	socksRepCmdNotSupported  = 0x07
	socksRepAddrNotSupported = 0x08
)

// handleSOCKS5WithReader 处理一个 SOCKS5 连接。
//
// 版本字节已经被探测逻辑消费，握手报文的剩余部分从 reader 读取
// （reader 可能已缓冲了部分数据，因此不能直接读裸连接）。
func (s *Server) handleSOCKS5WithReader(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
	// 1. 方法协商：VER(已读) | NMETHODS | METHODS...
	nMethods, err := reader.ReadByte()
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksReadNMethods), err)
	}
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(reader, methods); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksReadMethods), err)
	}

	// 本实现只支持“无认证”。
	if !containsByte(methods, socksAuthNone) {
		_, _ = conn.Write([]byte{socks5Version, socksAuthNoAcceptable})
		return errors.New(i18n.T(i18n.KeyErrProxySocksNoAuth))
	}
	if _, err := conn.Write([]byte{socks5Version, socksAuthNone}); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksReplyAuth), err)
	}

	// 2. 请求：VER | CMD | RSV | ATYP | DST.ADDR | DST.PORT
	header := make([]byte, 4)
	if _, err := io.ReadFull(reader, header); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksReadHeader), err)
	}
	if header[0] != socks5Version {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksBadVersion), header[0])
	}

	cmd := header[1]
	atyp := header[3]

	host, err := readSOCKSAddr(reader, atyp)
	if err != nil {
		_ = s.sendSOCKSReply(conn, socksRepAddrNotSupported)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksParseAddr), err)
	}
	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(reader, portBuf); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksReadPort), err)
	}
	port := binary.BigEndian.Uint16(portBuf)
	target := net.JoinHostPort(host, strconv.Itoa(int(port)))

	if cmd != socksCmdConnect {
		_ = s.sendSOCKSReply(conn, socksRepCmdNotSupported)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksBadCommand), cmd)
	}

	// 3. 直连目标（忽略环境代理）。
	dialCtx, cancel := context.WithTimeout(ctx, s.DialTimeout)
	defer cancel()
	if s.isSelfTarget(dialCtx, target) {
		_ = s.sendSOCKSReply(conn, socksRepGeneralFailure)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), target)
	}
	remote, err := s.dialer.DialContext(dialCtx, "tcp", target)
	if err != nil {
		s.recordDialError(ctx, "socks5", conn, target)
		_ = s.sendSOCKSReply(conn, socksReplyForDialError(err))
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksDial), target, err)
	}
	if s.isSelfConn(remote) {
		_ = remote.Close()
		_ = s.sendSOCKSReply(conn, socksRepGeneralFailure)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), target)
	}
	if !s.trackRemote(remote) {
		_ = remote.Close()
		return context.Canceled
	}
	defer s.untrackRemote(remote)
	remote = s.wrapRemote(remote)
	defer func() { _ = remote.Close() }()

	// 4. 回复成功。BND.ADDR/BND.PORT 填 0 即可，多数客户端不校验。
	if err := s.sendSOCKSReply(conn, socksRepSuccess); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySocksReplySuccess), err)
	}

	s.logf(i18n.T(i18n.KeyLogProxySOCKS5Relay), conn.RemoteAddr(), target)
	if !s.beginRelay(conn, remote) {
		return context.Canceled
	}
	start := time.Now()
	up, down := netutil.RelayReaderCount(conn, reader, remote)
	s.Stats().AddUp(up)
	s.Stats().AddDown(down)
	s.logEvent("close", "socks5", conn.RemoteAddr().String(), target, up, down, time.Since(start).Milliseconds(), connIDFromContext(ctx))
	return nil
}

// sendSOCKSReply 发送一个 SOCKS5 应答，绑定地址固定为 0.0.0.0:0。
func (s *Server) sendSOCKSReply(conn net.Conn, rep byte) error {
	reply := []byte{
		socks5Version, rep, 0x00, socksAddrIPv4,
		0, 0, 0, 0, // BND.ADDR
		0, 0, // BND.PORT
	}
	_, err := conn.Write(reply)
	return err
}

// readSOCKSAddr 根据地址类型读取目标主机。
func readSOCKSAddr(r *bufio.Reader, atyp byte) (string, error) {
	switch atyp {
	case socksAddrIPv4:
		buf := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil
	case socksAddrIPv6:
		buf := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil
	case socksAddrDomain:
		n, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return "", err
		}
		return string(buf), nil
	default:
		return "", fmt.Errorf(i18n.T(i18n.KeyErrProxySocksBadAddrType), atyp)
	}
}

// socksReplyForDialError 把出站拨号错误映射为合适的 SOCKS5 应答码。
func socksReplyForDialError(err error) byte {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return socksRepHostUnreach
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return socksRepConnRefused
		}
	}
	return socksRepGeneralFailure
}

func containsByte(bs []byte, target byte) bool {
	for _, b := range bs {
		if b == target {
			return true
		}
	}
	return false
}
