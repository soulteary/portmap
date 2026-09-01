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

// Package proxy 提供同时支持 SOCKS5 与 HTTP 的应用层代理服务。
//
// 同一个监听端口通过窥探连接首字节自动区分两种协议，所有出站连接均使用
// 纯粹的 net.Dialer 直连目标地址，完全忽略 HTTP_PROXY / HTTPS_PROXY /
// ALL_PROXY / NO_PROXY 等环境变量，不依赖任何上游代理。
package proxy

import (
	"bufio"
	"errors"
	"log"
	"net"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
)

// 默认参数。
const (
	defaultDialTimeout = 30 * time.Second
	defaultKeepAlive   = 30 * time.Second
)

// Server 是同时支持 SOCKS5 与 HTTP 的代理服务。
//
// 同一个监听端口通过窥探连接首字节自动区分两种协议：
//   - 首字节为 0x05 时按 SOCKS5 处理；
//   - 否则按 HTTP/HTTPS 代理处理。
type Server struct {
	// Addr 是监听地址，形如 "127.0.0.1:1080"。
	Addr string

	// DialTimeout 是出站连接超时时间。
	DialTimeout time.Duration

	// Logger 用于输出日志，为 nil 时使用标准库默认 logger。
	Logger *log.Logger

	dialer   Dialer
	listener net.Listener
}

// New 创建一个代理服务。所有出站连接均直连，忽略环境代理。
func New(addr string) *Server {
	return &Server{
		Addr:        addr,
		DialTimeout: defaultDialTimeout,
	}
}

// ListenAndServe 开始监听并处理连接，阻塞直到出错或被关闭。
func (s *Server) ListenAndServe() error {
	if s.DialTimeout <= 0 {
		s.DialTimeout = defaultDialTimeout
	}
	if s.dialer == nil {
		s.dialer = NewDirectDialer(s.DialTimeout, defaultKeepAlive)
	}

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	s.listener = ln
	s.logf(i18n.T(i18n.KeyLogProxyStarted), ln.Addr())

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			s.logf(i18n.T(i18n.KeyLogProxyAcceptFailed), err)
			continue
		}
		go s.serveConn(conn)
	}
}

// Close 关闭监听器，停止接受新连接。
func (s *Server) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// serveConn 处理单个连接：先探测协议，再分发到对应处理器。
func (s *Server) serveConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	reader := bufio.NewReader(conn)
	// Peek 一个字节用于协议探测，不会消费它。
	prefix, err := reader.Peek(1)
	if err != nil {
		s.logf(i18n.T(i18n.KeyLogProxyDetectFailed), conn.RemoteAddr(), err)
		return
	}

	if prefix[0] == socks5Version {
		// 消费掉版本字节，交给 SOCKS5 处理器（它从 NMETHODS 继续）。
		if _, err := reader.ReadByte(); err != nil {
			return
		}
		if err := s.handleSOCKS5WithReader(conn, reader); err != nil {
			s.logf(i18n.T(i18n.KeyLogProxySOCKS5Failed), conn.RemoteAddr(), err)
		}
		return
	}

	if err := s.handleHTTP(conn, reader); err != nil {
		s.logf(i18n.T(i18n.KeyLogProxyHTTPFailed), conn.RemoteAddr(), err)
	}
}

func (s *Server) logf(format string, args ...any) {
	if s.Logger != nil {
		s.Logger.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}
