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

package netutil

import (
	"bytes"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

type deadlineFailureConn struct {
	net.Conn
	input  *strings.Reader
	output bytes.Buffer
	calls  int
	failAt int
	err    error
}

func (c *deadlineFailureConn) Read(p []byte) (int, error)  { return c.input.Read(p) }
func (c *deadlineFailureConn) Write(p []byte) (int, error) { return c.output.Write(p) }
func (c *deadlineFailureConn) SetDeadline(time.Time) error {
	c.calls++
	if c.calls == c.failAt {
		return c.err
	}
	return nil
}
func (c *deadlineFailureConn) SetReadDeadline(t time.Time) error  { return c.SetDeadline(t) }
func (c *deadlineFailureConn) SetWriteDeadline(t time.Time) error { return c.SetDeadline(t) }
func TestIdleDeadlineFailures(t *testing.T) {
	for _, write := range []bool{false, true} {
		for _, failAt := range []int{1, 2} {
			t.Run(string(rune('a'+failAt))+map[bool]string{true: "write", false: "read"}[write], func(t *testing.T) {
				want := errors.New("deadline failed")
				raw := &deadlineFailureConn{input: strings.NewReader("x"), failAt: failAt, err: want}
				c := &IdleConn{Conn: raw}
				other := &IdleConn{Conn: &deadlineFailureConn{input: strings.NewReader("")}}
				ShareIdleTimeout(c, other, time.Second)
				var n int
				var err error
				if write {
					n, err = c.Write([]byte("x"))
				} else {
					n, err = c.Read(make([]byte, 1))
				}
				if !errors.Is(err, want) {
					t.Fatalf("error=%v", err)
				}
				if failAt == 1 && n != 0 {
					t.Fatal("I/O performed after deadline failure")
				}
				if failAt == 2 && n != 1 {
					t.Fatal("successful I/O count lost")
				}
			})
		}
	}
}
func TestStandaloneIdleReadAndDisabledDeadlines(t *testing.T) {
	raw := &deadlineFailureConn{input: strings.NewReader("ok")}
	c := &IdleConn{Conn: raw, Timeout: time.Second}
	data, err := io.ReadAll(c)
	if err != nil || string(data) != "ok" {
		t.Fatalf("%q %v", data, err)
	}
	if raw.calls == 0 {
		t.Fatal("read deadline not armed")
	}
	c.Timeout = 0
	before := raw.calls
	if _, err := c.Write([]byte("ok")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Read(make([]byte, 1)); err != io.EOF {
		t.Fatal(err)
	}
	if raw.calls != before {
		t.Fatal("disabled timeout updated deadline")
	}
	g := idleGroup{}
	if err := g.refreshDeadline(); err != nil {
		t.Fatal(err)
	}
}
func TestPooledCopyPreservesPayloadAndError(t *testing.T) {
	raw := &deadlineFailureConn{}
	n, err := CopyAndCloseWrite(raw, struct{ io.Reader }{strings.NewReader("payload")})
	if err != nil || n != 7 || raw.output.String() != "payload" {
		t.Fatalf("copy=%d %v %q", n, err, raw.output.String())
	}
	want := errors.New("reader failed")
	n, err = CopyAndCloseWrite(raw, errorReader{want})
	if n != 0 || !errors.Is(err, want) {
		t.Fatalf("copy=%d %v", n, err)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }
func TestRelayReaderCountTransfersBothDirections(t *testing.T) {
	a := &deadlineFailureConn{input: strings.NewReader("down")}
	b := &deadlineFailureConn{input: strings.NewReader("reply")}
	up, down := RelayReaderCount(a, strings.NewReader("request"), b)
	if up != 7 || down != 5 || a.output.String() != "reply" || b.output.String() != "request" {
		t.Fatalf("up=%d down=%d", up, down)
	}
}
func TestTCPIdleArmAndRetryDecisions(t *testing.T) {
	g := &tcpIdleState{start: time.Now(), timeout: time.Second}
	c := &tcpIdleConn{group: g}
	want := errors.New("set failed")
	if err := c.arm(&c.read, func(time.Time) error { return want }, false); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if c.read.due.Load() != 0 {
		t.Fatal("failed deadline recorded")
	}
	c.read.due.Store(g.deadline())
	if err := c.arm(&c.read, func(time.Time) error { t.Fatal("already armed"); return nil }, true); err != nil {
		t.Fatal(err)
	}
	if c.retry(want, &c.read) {
		t.Fatal("non-timeout retried")
	}
}
