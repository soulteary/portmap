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
	"log"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUpstreamTargetsUseRemoteNamespace(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	for _, scheme := range []string{UpstreamSchemeSSH, UpstreamSchemeHTTP, UpstreamSchemeSOCKS5} {
		t.Run(scheme, func(t *testing.T) {
			srv := New(ln.Addr().String())
			srv.listener = ln
			srv.Upstream = &UpstreamConfig{Scheme: scheme, Addr: "upstream.example:10024"}
			if srv.isSelfTarget(context.Background(), ln.Addr().String()) {
				t.Fatal("remote target was compared against local listener")
			}
			selfConn := srv.isSelfConn(&memoryConn{remote: ln.Addr()})
			if selfConn != (scheme != UpstreamSchemeSSH) {
				t.Fatal("incorrect transport self-connection check")
			}
			srv.Upstream.Addr = ln.Addr().String()
			if !srv.isSelfTarget(context.Background(), "application.example:10024") {
				t.Fatal("upstream endpoint may point at the local listener")
			}

		})
	}
}

// The test dialer models a remote namespace by mapping the requested address to
// a different local backend. No SSH server or external endpoint is needed.
func startNamespaceProxy(t *testing.T) string {
	t.Helper()
	backend, stop := startBackend(t)
	t.Cleanup(stop)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	srv := New(addr)
	srv.listener = ln
	srv.Upstream = &UpstreamConfig{Scheme: UpstreamSchemeSSH, Addr: "upstream.example:10024"}
	srv.Logger = log.New(io.Discard, "", 0)
	srv.dialer = dialFunc(func(ctx context.Context, network, target string) (net.Conn, error) {
		if target != addr {
			return nil, fmt.Errorf("unexpected remote target: %s", target)
		}
		return (&net.Dialer{}).DialContext(ctx, network, backend)
	})
	done := make(chan error, 1)
	go func() { done <- srv.acceptConnections(ln) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			t.Error(err)
		}
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	return addr
}

func TestProxySSHIdenticalLocalAndRemoteTargets(t *testing.T) {
	for _, mode := range []string{"http", "connect", "socks5"} {
		t.Run(mode, func(t *testing.T) {
			addr := startNamespaceProxy(t)
			conn := namespaceProxyConn(t, mode, addr)
			defer func() { _ = conn.Close() }()
			if mode == "http" {
				_, err := fmt.Fprintf(conn, "GET http://%s/ HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", addr, addr)
				if err != nil {
					t.Fatal(err)
				}
				body, err := io.ReadAll(conn)
				if err != nil || !strings.Contains(string(body), "hello from backend") {
					t.Fatalf("response=%q err=%v", body, err)
				}
				return
			}
			assertHTTPGetOverConn(t, conn, addr)
		})
	}
}

func namespaceProxyConn(t *testing.T, mode, addr string) net.Conn {
	t.Helper()
	var conn net.Conn
	var err error
	if mode == "socks5" {
		conn, err = socks5Dial(addr, addr)
	} else {
		conn, err = net.DialTimeout("tcp", addr, time.Second)
	}
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if mode == "connect" {
		if _, err := fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", addr, addr); err != nil {
			_ = conn.Close()
			t.Fatal(err)
		}
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			_ = conn.Close()
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = conn.Close()
			t.Fatalf("CONNECT status=%d", resp.StatusCode)
		}
	}
	return conn
}
