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
