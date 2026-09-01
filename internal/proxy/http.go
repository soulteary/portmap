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
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/netutil"
)

// handleHTTP 处理一个 HTTP/HTTPS 代理连接。
//
// reader 已经包裹了原始连接，并且其缓冲区开头包含用于协议探测时
// “塞回去”的字节，因此可以直接从 reader 解析完整的 HTTP 请求。
func (s *Server) handleHTTP(conn net.Conn, reader *bufio.Reader) error {
	req, err := http.ReadRequest(reader)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPParseRequest), err)
	}

	if req.Method == http.MethodConnect {
		return s.handleConnect(conn, reader, req)
	}
	return s.handlePlainHTTP(conn, reader, req)
}

// handleConnect 处理 HTTPS CONNECT 转发。
func (s *Server) handleConnect(conn net.Conn, reader *bufio.Reader, req *http.Request) error {
	target := req.Host
	if !hasPort(target) {
		target = net.JoinHostPort(target, "443")
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.DialTimeout)
	defer cancel()
	if s.isSelfTarget(ctx, target) {
		writeHTTPError(conn, http.StatusLoopDetected)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), target)
	}
	remote, err := s.dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		writeHTTPError(conn, http.StatusBadGateway)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPConnectDial), target, err)
	}
	if s.isSelfConn(remote) {
		_ = remote.Close()
		writeHTTPError(conn, http.StatusLoopDetected)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), target)
	}
	remote = s.wrapRemote(remote)
	defer func() { _ = remote.Close() }()

	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPConnectReply), err)
	}

	s.logf(i18n.T(i18n.KeyLogProxyHTTPConnect), conn.RemoteAddr(), target)
	s.beginRelay(conn)
	netutil.RelayReader(conn, reader, remote)
	return nil
}

// handlePlainHTTP 处理普通（明文）HTTP 代理请求。
func (s *Server) handlePlainHTTP(conn net.Conn, reader *bufio.Reader, req *http.Request) error {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if !hasPort(host) {
		host = net.JoinHostPort(host, "80")
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.DialTimeout)
	defer cancel()
	if s.isSelfTarget(ctx, host) {
		writeHTTPError(conn, http.StatusLoopDetected)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), host)
	}
	remote, err := s.dialer.DialContext(ctx, "tcp", host)
	if err != nil {
		writeHTTPError(conn, http.StatusBadGateway)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPDial), host, err)
	}
	if s.isSelfConn(remote) {
		_ = remote.Close()
		writeHTTPError(conn, http.StatusLoopDetected)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), host)
	}
	remote = s.wrapRemote(remote)
	defer func() { _ = remote.Close() }()
	s.beginRelay(conn)

	// 改写为源站可识别的相对路径请求，并清理逐跳首部。
	req.RequestURI = ""
	stripHopByHopHeaders(req.Header)
	req.Header.Set("Connection", "close")

	if err := req.Write(remote); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPForward), host, err)
	}

	s.logf(i18n.T(i18n.KeyLogProxyHTTPPlain), req.Method, conn.RemoteAddr(), host)

	// 把源站响应原样回传给客户端。
	if _, err := io.Copy(conn, remote); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPRelayResp), err)
	}
	return nil
}

// hopByHopHeaders 是 RFC 7230 定义的逐跳首部，转发时应当移除。
var hopByHopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

func stripHopByHopHeaders(h http.Header) {
	for _, key := range hopByHopHeaders {
		h.Del(key)
	}
}

func writeHTTPError(conn net.Conn, code int) {
	_, _ = fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\n\r\n", code, http.StatusText(code))
}

func hasPort(host string) bool {
	if host == "" {
		return false
	}
	// IPv6 字面量形如 [::1]:80。
	if strings.HasPrefix(host, "[") {
		return strings.Contains(host, "]:")
	}
	return strings.LastIndex(host, ":") > strings.LastIndex(host, "]")
}
