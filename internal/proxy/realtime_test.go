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
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func awaitBytes(t *testing.T, srv *Server, up, down int64) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s := srv.Snapshot()
		if s.UpBytes >= up && s.DownBytes >= down && s.ActiveConns > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("not counted while open: %+v", srv.Snapshot())
}
func TestTunnelsCountBeforeClose(t *testing.T) {
	for _, proto := range []string{"socks5", "connect"} {
		t.Run(proto, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = ln.Close() }()
			go func() {
				c, err := ln.Accept()
				if err != nil {
					return
				}
				defer func() { _ = c.Close() }()
				_, _ = io.Copy(c, c)
			}()
			addr, srv, stop := startTestProxyRef(t)
			defer stop()
			var conn net.Conn
			if proto == "socks5" {
				conn, err = socks5Dial(addr, ln.Addr().String())
			} else {
				conn, err = net.Dial("tcp", addr)
				if err == nil {
					_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", ln.Addr(), ln.Addr())
					if err == nil {
						var resp *http.Response
						resp, err = http.ReadResponse(bufio.NewReader(conn), nil)
						if err == nil && resp.StatusCode != 200 {
							t.Fatalf("status %d", resp.StatusCode)
						}
					}
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.Close() }()
			_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
			if _, err := conn.Write([]byte("ping")); err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(conn, make([]byte, 4)); err != nil {
				t.Fatal(err)
			}
			awaitBytes(t, srv, 4, 4)
		})
	}
}
func TestPlainHTTPCountsStreamingResponse(t *testing.T) {
	release := make(chan struct{})
	backend := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "first")
		w.(http.Flusher).Flush()
		<-release
	})}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = backend.Serve(ln) }()
	defer func() { close(release); _ = backend.Close() }()
	addr, srv, stop := startTestProxyRef(t)
	defer stop()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, err = fmt.Fprintf(conn, "GET http://%s/ HTTP/1.1\r\nHost: %s\r\n\r\n", ln.Addr(), ln.Addr())
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(resp.Body, make([]byte, 5)); err != nil {
		t.Fatal(err)
	}
	awaitBytes(t, srv, 1, 5)
}

func TestProxyStatsLabel(t *testing.T) {
	if label := New("127.0.0.1:8118").StatsLabel(); label != "proxy 127.0.0.1:8118" {
		t.Fatal(label)
	}
}
