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
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/netutil"
)

// handleHTTP 处理一个 HTTP/HTTPS 代理连接。
//
// reader 已经包裹了原始连接，并且其缓冲区开头包含用于协议探测时
// “塞回去”的字节，因此可以直接从 reader 解析完整的 HTTP 请求。
func (s *Server) handleHTTP(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
	req, err := http.ReadRequest(reader)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPParseRequest), err)
	}

	if req.Method == http.MethodConnect {
		return s.handleConnect(ctx, conn, reader, req)
	}
	return s.handlePlainHTTP(ctx, conn, reader, req)
}

// handleConnect 处理 HTTPS CONNECT 转发。
func (s *Server) handleConnect(ctx context.Context, conn net.Conn, reader *bufio.Reader, req *http.Request) error {
	target := req.Host
	if !hasPort(target) {
		target = net.JoinHostPort(target, "443")
	}

	dialCtx, cancel := context.WithTimeout(ctx, s.DialTimeout)
	defer cancel()
	if s.isSelfTarget(dialCtx, target) {
		writeHTTPError(conn, http.StatusLoopDetected)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), target)
	}
	remote, err := s.dialer.DialContext(dialCtx, "tcp", target)
	if err != nil {
		writeHTTPError(conn, http.StatusBadGateway)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPConnectDial), target, err)
	}
	if s.isSelfConn(remote) {
		_ = remote.Close()
		writeHTTPError(conn, http.StatusLoopDetected)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), target)
	}
	if !s.trackRemote(remote) {
		_ = remote.Close()
		return context.Canceled
	}
	defer s.untrackRemote(remote)
	remote = s.wrapRemote(remote)
	defer func() { _ = remote.Close() }()

	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPConnectReply), err)
	}

	s.logf(i18n.T(i18n.KeyLogProxyHTTPConnect), conn.RemoteAddr(), target)
	if !s.beginRelay(conn, remote) {
		return context.Canceled
	}
	netutil.RelayReader(conn, reader, remote)
	return nil
}

// handlePlainHTTP 处理普通（明文）HTTP 代理请求。
func (s *Server) handlePlainHTTP(ctx context.Context, conn net.Conn, reader *bufio.Reader, req *http.Request) error {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}
	if !hasPort(host) {
		host = net.JoinHostPort(host, "80")
	}

	dialCtx, cancel := context.WithTimeout(ctx, s.DialTimeout)
	defer cancel()
	if s.isSelfTarget(dialCtx, host) {
		writeHTTPError(conn, http.StatusLoopDetected)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), host)
	}
	remote, err := s.dialer.DialContext(dialCtx, "tcp", host)
	if err != nil {
		writeHTTPError(conn, http.StatusBadGateway)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPDial), host, err)
	}
	if s.isSelfConn(remote) {
		_ = remote.Close()
		writeHTTPError(conn, http.StatusLoopDetected)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxySelfTarget), host)
	}
	if !s.trackRemote(remote) {
		_ = remote.Close()
		return context.Canceled
	}
	defer s.untrackRemote(remote)
	remote = s.wrapRemote(remote)
	defer func() { _ = remote.Close() }()
	if !s.beginRelay(conn, remote) {
		return context.Canceled
	}

	// 改写为源站可识别的相对路径请求，并清理逐跳首部。
	req.RequestURI = ""
	requestConnectionOptions := connectionOptionNames(req.Header)
	stripHopByHopHeaders(req.Header)
	req.Body = filterTrailers(req.Body, req.Trailer, requestConnectionOptions)
	appendVia(req.Header, req.ProtoMajor, req.ProtoMinor)
	req.Close = true
	req.Header.Set("Connection", "close")

	if err := req.Write(remote); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPForward), host, err)
	}

	s.logf(i18n.T(i18n.KeyLogProxyHTTPPlain), req.Method, conn.RemoteAddr(), host)

	// 解析响应后清理响应侧逐跳首部并追加 Via；1xx（101 除外）可能在最终
	// 响应之前出现，因此需要逐个转发。
	remoteReader := newProxyResponseReader(remote)
	for {
		resp, connectionOptions, err := remoteReader.read(req)
		if err != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPRelayResp), err)
		}
		for _, key := range connectionOptions {
			resp.Header.Del(key)
		}
		stripHopByHopHeaders(resp.Header)
		resp.Body = filterTrailers(resp.Body, resp.Trailer, connectionOptions)
		appendVia(resp.Header, resp.ProtoMajor, resp.ProtoMinor)
		informational := resp.StatusCode >= 100 && resp.StatusCode < 200 && resp.StatusCode != http.StatusSwitchingProtocols
		if informational {
			// ReadResponse 会把上游 Connection: close 保存到 Close 字段；即使
			// 已删除原始首部，Response.Write 仍会据此重新生成该逐跳首部。
			// 1xx 后还要在同一连接上传递最终响应，因此必须显式清除。
			resp.Close = false
		} else {
			resp.Close = true
			resp.Header.Set("Connection", "close")
		}
		writeErr := resp.Write(conn)
		_ = resp.Body.Close()
		if writeErr != nil {
			return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPRelayResp), writeErr)
		}
		if !informational {
			break
		}
	}
	return nil
}

const maxProxyResponseHeaderBytes = 10 << 20

// replayReader can prepend bytes without wrapping its previous state in another
// reader. proxyResponseReader uses it to replay a response head while retaining
// any bytes that bufio already fetched from the following response or body.
type replayReader struct {
	source io.Reader
	prefix []byte
}

func (r *replayReader) Read(p []byte) (int, error) {
	if len(r.prefix) > 0 {
		n := copy(p, r.prefix)
		r.prefix = r.prefix[n:]
		return n, nil
	}
	return r.source.Read(p)
}

func (r *replayReader) prepend(parts ...[]byte) {
	size := len(r.prefix)
	for _, part := range parts {
		size += len(part)
	}
	prefix := make([]byte, 0, size)
	for _, part := range parts {
		prefix = append(prefix, part...)
	}
	prefix = append(prefix, r.prefix...)
	r.prefix = prefix
}

type proxyResponseReader struct {
	source *replayReader
	reader *bufio.Reader
}

// trailerFilteringBody removes hop-by-hop trailers after net/http has consumed
// the chunked body and populated Response.Trailer, but before Response.Write
// serializes those trailers downstream.
type trailerFilteringBody struct {
	io.ReadCloser
	filter func()
}

func (b *trailerFilteringBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil && b.filter != nil {
		b.filter()
		b.filter = nil
	}
	return n, err
}

func newProxyResponseReader(source io.Reader) *proxyResponseReader {
	replay := &replayReader{source: source}
	return &proxyResponseReader{
		source: replay,
		reader: bufio.NewReader(replay),
	}
}

// read 在交给 net/http 解析前保留原始 Connection 选项。
// http.ReadResponse 遇到 Connection: close 时会删除整个 Connection 首部，
// 若其中还点名了其他逐跳字段，解析完成后将无法再知道应删除哪些字段。
func (r *proxyResponseReader) read(req *http.Request) (*http.Response, []string, error) {
	head, err := readResponseHead(r.reader)
	if err != nil {
		return nil, nil, err
	}

	// Save bytes already fetched beyond this head, then reset the same bufio
	// reader over a flat replay buffer. The replay source also contributes any
	// prefix it has not delivered yet, so no bytes are lost and repeated 1xx
	// responses cannot build an ever-deeper MultiReader chain.
	buffered := make([]byte, r.reader.Buffered())
	if _, err := io.ReadFull(r.reader, buffered); err != nil {
		return nil, nil, err
	}
	r.source.prepend(head, buffered)
	r.reader.Reset(r.source)

	resp, err := http.ReadResponse(r.reader, req)
	if err != nil {
		return nil, nil, err
	}

	tp := textproto.NewReader(bufio.NewReader(bytes.NewReader(head)))
	if _, err := tp.ReadLine(); err != nil {
		_ = resp.Body.Close()
		return nil, nil, err
	}
	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		_ = resp.Body.Close()
		return nil, nil, err
	}
	return resp, connectionOptionNames(http.Header(mimeHeader)), nil
}

func readResponseHead(reader *bufio.Reader) ([]byte, error) {
	var head bytes.Buffer
	continuedLine := false
	for {
		fragment, err := reader.ReadSlice('\n')
		if head.Len()+len(fragment) > maxProxyResponseHeaderBytes {
			return nil, fmt.Errorf("proxy response headers exceed %d bytes", maxProxyResponseHeaderBytes)
		}
		_, _ = head.Write(fragment)
		if err == bufio.ErrBufferFull {
			continuedLine = true
			continue
		}
		if err != nil {
			return nil, err
		}
		blankLine := !continuedLine && (bytes.Equal(fragment, []byte("\r\n")) || bytes.Equal(fragment, []byte("\n")))
		continuedLine = false
		if blankLine {
			return head.Bytes(), nil
		}
	}
}

func filterTrailers(body io.ReadCloser, trailer http.Header, connectionOptions []string) io.ReadCloser {
	filter := func() {
		for _, key := range connectionOptions {
			trailer.Del(key)
		}
		stripHopByHopHeaders(trailer)
	}

	// Remove declarations before Response.Write emits its header, then repeat
	// after EOF because net/http repopulates Trailer while consuming the body.
	filter()
	if body != nil && body != http.NoBody {
		return &trailerFilteringBody{ReadCloser: body, filter: filter}
	}
	return body
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
	// Connection 可以列出额外的逐跳字段名，必须在删除 Connection 自身前
	// 逐一删除（RFC 9110 §7.6.1）。
	for _, key := range connectionOptionNames(h) {
		h.Del(key)
	}
	for _, key := range hopByHopHeaders {
		h.Del(key)
	}
}

func connectionOptionNames(h http.Header) []string {
	var keys []string
	for _, value := range h.Values("Connection") {
		for _, token := range strings.Split(value, ",") {
			if key := textproto.TrimString(token); key != "" {
				keys = append(keys, key)
			}
		}
	}
	return keys
}

func appendVia(h http.Header, major, minor int) {
	h.Add("Via", fmt.Sprintf("%d.%d portmap", major, minor))
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
