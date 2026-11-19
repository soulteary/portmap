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

package forward

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

func TestTCPCountsBeforeClose(t *testing.T) {
	for _, idle := range []time.Duration{0, time.Second} {
		t.Run(idle.String(), func(t *testing.T) {
			target, stop := startEchoServer(t)
			defer stop()
			addr, srv, _, _ := startSSHForward(t, Config{Target: target, IdleTimeout: idle})
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = conn.Close() }()
			_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
			payload := []byte("still connected")
			if _, err := conn.Write(payload); err != nil {
				t.Fatal(err)
			}
			if _, err := io.ReadFull(conn, make([]byte, len(payload))); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(time.Second)
			for time.Now().Before(deadline) {
				s := srv.Snapshot()
				if s.UpBytes == int64(len(payload)) && s.DownBytes == int64(len(payload)) && s.ActiveConns == 1 {
					return
				}
				time.Sleep(time.Millisecond)
			}
			t.Fatalf("not counted while open: %+v", srv.Snapshot())
		})
	}
}

func TestForwardStatsLabel(t *testing.T) {
	s := New(Config{Network: "tcp", Listen: "127.0.0.1:31008", Target: "127.0.0.1:8080"})
	if s.StatsLabel() != "forward tcp://127.0.0.1:31008 -> 127.0.0.1:8080" {
		t.Fatal(s.StatsLabel())
	}
}

func TestSSHCountsBeforeClose(t *testing.T) {
	target, stop := startEchoServer(t)
	defer stop()
	ssh := StartSSH(t, func(_ context.Context, _ string) (string, error) { return target, nil })
	addr, srv, _, _ := startSSHForward(t, Config{Target: "127.0.0.1:31008", Upstream: ssh.Config(), IdleTimeout: time.Second})
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte("ssh")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, make([]byte, 3)); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s := srv.Snapshot()
		if s.UpBytes == 3 && s.DownBytes == 3 && s.ActiveConns == 1 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("SSH not counted while open: %+v", srv.Snapshot())
}
