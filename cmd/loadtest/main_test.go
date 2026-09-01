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

package main

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestParseFlagsValidatesTerminationAndLimits(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "zero duration", args: []string{"-duration", "0s"}},
		{name: "negative duration", args: []string{"-duration", "-1s"}},
		{name: "negative warmup", args: []string{"-warmup", "-1s"}},
		{name: "negative max conns", args: []string{"-max-conns", "-1"}},
		{name: "negative idle timeout", args: []string{"-idle-timeout", "-1s"}},
		{name: "oversized udp datagram", args: []string{"-proto", "udp", "-payload", "65508"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseFlags(tc.args); err == nil {
				t.Fatalf("parseFlags(%v) 应返回错误", tc.args)
			}
		})
	}
}

func TestFixedRequestModeStopsAtDurationWhenTargetFails(t *testing.T) {
	tests := []struct {
		name  string
		proto string
		mode  string
		addr  func(*testing.T) string
	}{
		{
			name:  "tcp connrate",
			proto: "tcp",
			mode:  "connrate",
			addr: func(t *testing.T) string {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				addr := ln.Addr().String()
				_ = ln.Close()
				return addr
			},
		},
		{
			name:  "udp throughput",
			proto: "udp",
			mode:  "throughput",
			addr: func(t *testing.T) string {
				conn, err := net.ListenPacket("udp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				addr := conn.LocalAddr().String()
				_ = conn.Close()
				return addr
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := &options{
				proto:    tc.proto,
				mode:     tc.mode,
				conns:    1,
				duration: 80 * time.Millisecond,
				requests: 1,
				payload:  32,
				external: tc.addr(t),
			}
			started := time.Now()
			if err := runWithOutput(o, io.Discard, io.Discard); err != nil {
				t.Fatalf("runWithOutput 返回错误: %v", err)
			}
			if elapsed := time.Since(started); elapsed > time.Second {
				t.Fatalf("固定请求模式在目标失败时未按 duration 结束: %s", elapsed)
			}
		})
	}
}

func TestRoundTripDeadlineFailuresAreRecorded(t *testing.T) {
	t.Run("tcp", func(t *testing.T) {
		serverConn, clientConn := net.Pipe()
		defer func() { _ = serverConn.Close() }()
		defer func() { _ = clientConn.Close() }()
		payload := makePayload(32)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		received := make(chan struct{})
		go func() {
			buf := make([]byte, len(payload))
			_, _ = io.ReadFull(serverConn, buf)
			close(received)
			<-ctx.Done()
		}()

		s := &stats{}
		if _, ok := tcpRoundTrip(ctx, clientConn, payload, make([]byte, len(payload)), s); ok {
			t.Fatal("静默 TCP 目标不应返回成功")
		}
		if got := s.errRead.Load(); got != 1 {
			t.Fatalf("截止时间触发的 TCP 读取失败数=%d，期望 1", got)
		}
		select {
		case <-received:
		case <-time.After(time.Second):
			t.Fatal("TCP 请求未实际发送到目标")
		}
	})

	t.Run("udp", func(t *testing.T) {
		serverConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
		if err != nil {
			t.Fatalf("监听 UDP 失败: %v", err)
		}
		defer func() { _ = serverConn.Close() }()
		clientConn, err := net.DialUDP("udp", nil, serverConn.LocalAddr().(*net.UDPAddr))
		if err != nil {
			t.Fatalf("连接 UDP 目标失败: %v", err)
		}
		defer func() { _ = clientConn.Close() }()
		payload := makePayload(32)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		received := make(chan struct{})
		go func() {
			buf := make([]byte, len(payload))
			_, _, _ = serverConn.ReadFromUDP(buf)
			close(received)
			<-ctx.Done()
		}()

		s := &stats{}
		if _, ok := udpRoundTrip(ctx, clientConn, payload, make([]byte, len(payload)+64), s); ok {
			t.Fatal("静默 UDP 目标不应返回成功")
		}
		if got := s.errRead.Load(); got != 1 {
			t.Fatalf("截止时间触发的 UDP 读取失败数=%d，期望 1", got)
		}
		select {
		case <-received:
		case <-time.After(time.Second):
			t.Fatal("UDP 请求未实际发送到目标")
		}
	})
}
