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
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/netutil"
)

// handleHTTP 处理一个 HTTP/HTTPS 代理连接。
//
// reader 已经包裹了原始连接，并且其缓冲区开头包含用于协议探测时
// “塞回去”的字节，因此可以直接从 reader 解析完整的 HTTP 请求。
func (s *Server) handleHTTP(ctx context.Context, conn net.Conn, reader *bufio.Reader) error {
	req, requestReader, err := readProxyRequest(reader)
	if err != nil {
		if errors.Is(err, errProxyRequestHeadersTooLarge) {
			writeHTTPError(conn, http.StatusRequestHeaderFieldsTooLarge)
		}
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPParseRequest), err)
	}
	reader = requestReader
	if err := ctx.Err(); err != nil {
		return err
	}
	if isUpgradeRequest(req) {
		writeHTTPError(conn, http.StatusNotImplemented)
		return errors.New(i18n.T(i18n.KeyErrProxyHTTPUpgradeUnsupported))
	}
	if !s.completeHandshake(conn) {
		return context.Canceled
	}
	ctx = context.WithValue(s.serverContext(), connIDContextKey{}, connIDFromContext(ctx))

	if req.Method == http.MethodConnect {
		return s.handleConnect(ctx, conn, reader, req)
	}
	return s.handlePlainHTTP(ctx, conn, reader, req)
}

func isUpgradeRequest(req *http.Request) bool {
	hasUpgradeValue := false
	for _, value := range req.Header.Values("Upgrade") {
		if strings.TrimSpace(value) != "" {
			hasUpgradeValue = true
			break
		}
	}
	if !hasUpgradeValue {
		return false
	}
	for _, option := range connectionOptionNames(req.Header) {
		if strings.EqualFold(option, "upgrade") {
			return true
		}
	}
	return false
}

const maxProxyRequestHeaderBytes = 1 << 20

var errProxyRequestHeadersTooLarge = fmt.Errorf("proxy request headers exceed %d bytes", maxProxyRequestHeaderBytes)

// readProxyRequest bounds the request line and MIME headers before handing the
// stream to net/http. It returns the replay reader so buffered request-body or
// CONNECT tunnel bytes remain available to the existing forwarding paths.
func readProxyRequest(reader *bufio.Reader) (*http.Request, *bufio.Reader, error) {
	head, err := readRequestHead(reader)
	if err != nil {
		return nil, nil, err
	}
	replay := bufio.NewReader(io.MultiReader(bytes.NewReader(head), reader))
	req, err := http.ReadRequest(replay)
	if err != nil {
		return nil, nil, err
	}
	return req, replay, nil
}

func readRequestHead(reader *bufio.Reader) ([]byte, error) {
	var head bytes.Buffer
	continuedLine := false
	for {
		fragment, err := reader.ReadSlice('\n')
		if head.Len()+len(fragment) > maxProxyRequestHeaderBytes {
			return nil, errProxyRequestHeadersTooLarge
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
		s.recordDialError(ctx, "http", conn, target)
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

	if !s.beginRelay(conn, remote) {
		return context.Canceled
	}
	if _, err := io.WriteString(conn, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPConnectReply), err)
	}

	s.logf(i18n.T(i18n.KeyLogProxyHTTPConnect), conn.RemoteAddr(), target)
	start := time.Now()
	up, down := netutil.RelayReaderCount(conn, reader, remote)
	s.Stats().AddUp(up)
	s.Stats().AddDown(down)
	s.logEvent("close", "http", conn.RemoteAddr().String(), target, up, down, time.Since(start).Milliseconds(), connIDFromContext(ctx))
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
		s.recordDialError(ctx, "http", conn, host)
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
	start := time.Now()
	// 改写为源站可识别的相对路径请求，并清理逐跳首部。
	req.RequestURI = ""
	requestConnectionOptions := connectionOptionNames(req.Header)
	stripHopByHopHeaders(req.Header)
	req.Body = filterTrailers(req.Body, req.Trailer, requestConnectionOptions)
	appendVia(req.Header, req.ProtoMajor, req.ProtoMinor)
	req.Close = true
	req.Header.Set("Connection", "close")

	// A client using Expect: 100-continue waits before sending the body. Request.Write
	// consumes that body before we start reading upstream responses, so forwarding the
	// expectation unchanged would deadlock when the origin sends the interim response.
	// A proxy may answer the expectation itself: acknowledge it immediately and remove
	// the header so the origin reads the subsequently forwarded body normally.
	downCounter := &countingWriter{w: conn}
	if req.ProtoAtLeast(1, 1) && removeHeaderToken(req.Header, "Expect", "100-continue") {
		if _, err := io.WriteString(downCounter, "HTTP/1.1 100 Continue\r\n\r\n"); err != nil {
			s.Stats().AddDown(downCounter.n)
			return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPRelayResp), err)
		}
	}

	upCounter := &countingWriter{w: remote}
	if err := req.Write(upCounter); err != nil {
		s.Stats().AddUp(upCounter.n)
		s.Stats().AddDown(downCounter.n)
		return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPForward), host, err)
	}
	s.Stats().AddUp(upCounter.n)

	s.logf(i18n.T(i18n.KeyLogProxyHTTPPlain), req.Method, conn.RemoteAddr(), host)

	// 解析响应后清理响应侧逐跳首部并追加 Via；1xx（101 除外）可能在最终
	// 响应之前出现，因此需要逐个转发。
	remoteReader := newProxyResponseReader(remote)
	for {
		resp, connectionOptions, err := remoteReader.read(req)
		if err != nil {
			s.Stats().AddDown(downCounter.n)
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
		writeErr := resp.Write(downCounter)
		_ = resp.Body.Close()
		if writeErr != nil {
			s.Stats().AddDown(downCounter.n)
			return fmt.Errorf(i18n.T(i18n.KeyErrProxyHTTPRelayResp), writeErr)
		}
		if !informational {
			break
		}
	}
	s.Stats().AddDown(downCounter.n)
	s.logEvent("close", "http", conn.RemoteAddr().String(), host, upCounter.n, downCounter.n, time.Since(start).Milliseconds(), connIDFromContext(ctx))
	return nil
}

// countingWriter 包装一个 io.Writer 并累计成功写出的字节数，用于明文 HTTP
// 代理路径统计上/下行流量（该路径不经 netutil.RelayReaderCount）。
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
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

func removeHeaderToken(header http.Header, name, want string) bool {
	found := false
	keptValues := make([]string, 0, len(header.Values(name)))
	for _, value := range header.Values(name) {
		keptTokens := make([]string, 0)
		for _, rawToken := range splitHeaderList(value) {
			token := textproto.TrimString(rawToken)
			if token == "" {
				continue
			}
			if strings.EqualFold(token, want) {
				found = true
				continue
			}
			keptTokens = append(keptTokens, token)
		}
		if len(keptTokens) > 0 {
			keptValues = append(keptValues, strings.Join(keptTokens, ", "))
		}
	}
	if !found {
		return false
	}
	header.Del(name)
	for _, value := range keptValues {
		header.Add(name, value)
	}
	return true
}

func splitHeaderList(value string) []string {
	var members []string
	start := 0
	quoted := false
	escaped := false
	for i := 0; i < len(value); i++ {
		switch {
		case escaped:
			escaped = false
		case quoted && value[i] == '\\':
			escaped = true
		case value[i] == '"':
			quoted = !quoted
		case value[i] == ',' && !quoted:
			members = append(members, textproto.TrimString(value[start:i]))
			start = i + 1
		}
	}
	members = append(members, textproto.TrimString(value[start:]))
	return members
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
