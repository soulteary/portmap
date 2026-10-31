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
	"io"
	"net"
	"testing"
	"time"
)

func tcpPair(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	a, err := net.DialTCP("tcp", nil, ln.Addr().(*net.TCPAddr))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	b, err := ln.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return a, b
}

func TestTCPIdleTunnelActivityAndExpiry(t *testing.T) {
	a, ap := tcpPair(t)
	b, bp := tcpPair(t)
	const idle = 120 * time.Millisecond
	x, y := TCPIdleConns(a, b, idle)
	done := make(chan struct{})
	go func() { Relay(x, y); close(done) }()
	// One-way traffic crosses multiple expired socket deadlines. A pending
	// reverse read must survive, then deliver reverse traffic without reconnecting.
	until := time.Now().Add(4 * idle)
	for time.Now().Before(until) {
		_ = ap.SetDeadline(time.Now().Add(time.Second))
		if _, err := ap.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
		_ = bp.SetReadDeadline(time.Now().Add(time.Second))
		if _, err := io.ReadFull(bp, make([]byte, 1)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(idle / 5)
	}
	if _, err := bp.Write([]byte("reverse")); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(ap, make([]byte, 7)); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	select {
	case <-done:
		if time.Since(started) < idle/2 {
			t.Fatal("expired before the final activity's idle timeout")
		}
	case <-time.After(4 * idle):
		t.Fatal("idle tunnel failed to expire")
	}
}

func TestTCPIdleHalfClose(t *testing.T) {
	a, ap := tcpPair(t)
	b, bp := tcpPair(t)
	x, y := TCPIdleConns(a, b, time.Second)
	done := make(chan struct{})
	go func() { Relay(x, y); close(done) }()
	if _, err := ap.Write([]byte("request")); err != nil {
		t.Fatal(err)
	}
	if err := ap.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	_ = bp.SetDeadline(time.Now().Add(time.Second))
	got, err := io.ReadAll(bp)
	if err != nil || string(got) != "request" {
		t.Fatalf("request=%q err=%v", got, err)
	}
	if _, err = bp.Write([]byte("response")); err != nil {
		t.Fatal(err)
	}
	_ = bp.CloseWrite()
	_ = ap.SetReadDeadline(time.Now().Add(time.Second))
	got, err = io.ReadAll(ap)
	if err != nil || string(got) != "response" {
		t.Fatalf("response=%q err=%v", got, err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("half-closed relay stuck")
	}
}

func TestTCPIdleCloseInterruptsRead(t *testing.T) {
	a, _ := tcpPair(t)
	b, _ := tcpPair(t)
	x, _ := TCPIdleConns(a, b, time.Hour)
	done := make(chan error, 1)
	go func() { _, err := x.Read(make([]byte, 1)); done <- err }()
	_ = x.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("closed read succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("close did not interrupt read")
	}
}

func TestTCPIdleDisabledPreservesFastPaths(t *testing.T) {
	a, _ := tcpPair(t)
	b, _ := tcpPair(t)
	x, y := TCPIdleConns(a, b, 0)
	if x != a || y != b {
		t.Fatal("disabled timeout must return raw TCP connections")
	}
}

func TestTCPIdleBlockedWriteSurvivesOtherDirectionActivity(t *testing.T) {
	a, ap := tcpPair(t)
	b, bp := tcpPair(t)
	if err := a.SetWriteBuffer(1024); err != nil {
		t.Fatal(err)
	}
	const idle = 100 * time.Millisecond
	x, y := TCPIdleConns(a, b, idle)
	payload := make([]byte, 4*1024*1024)
	fillRetryPayload(payload)
	type result struct {
		n   int
		err error
	}
	done := make(chan result, 1)
	blockedRead := make(chan error, 1)
	go func() { _, err := x.Read(make([]byte, 1)); blockedRead <- err }()
	go func() { n, err := x.Write(payload); done <- result{n, err} }()
	// Keep the tunnel active through b while a's write is backpressured. Its
	// native write can time out with a nonzero byte count; no bytes may be lost
	// or duplicated when retrying the remaining suffix.
	stop := make(chan struct{})
	activity := make(chan error, 1)
	go keepTCPIdleActive(bp, stop, activity, idle/5)
	readDone := make(chan struct{})
	go drainIdleActivity(y, readDone)
	time.Sleep(4 * idle)
	select {
	case r := <-done:
		t.Fatalf("active tunnel write ended early: %d %v", r.n, r.err)
	default:
	}
	assertIdleReadStillBlocked(t, blockedRead)
	if _, err := ap.Write([]byte("r")); err != nil {
		t.Fatal(err)
	}
	if err := <-blockedRead; err != nil {
		t.Fatalf("read after repeated shared deadline expiry: %v", err)
	}
	_ = ap.SetReadDeadline(time.Now().Add(5 * time.Second))
	received := make([]byte, len(payload))
	n, err := io.ReadFull(ap, received)
	if err != nil || n != len(payload) {
		t.Fatalf("drain=%d %v", n, err)
	}
	if !bytes.Equal(received, payload) {
		t.Fatal("partial write retry corrupted payload")
	}
	select {
	case r := <-done:
		if r.err != nil || r.n != len(payload) {
			t.Fatalf("write=%d %v", r.n, r.err)
		}
	case <-time.After(time.Second):
		t.Fatal("write stuck after drain")
	}
	close(stop)
	if err := <-activity; err != nil {
		t.Fatal(err)
	}
	_ = y.Close()
	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("read not cancelled")
	}
}

func TestTCPIdleExternalDeadlineReturns(t *testing.T) {
	a, _ := tcpPair(t)
	b, _ := tcpPair(t)
	x, _ := TCPIdleConns(a, b, time.Hour)
	done := make(chan error, 1)
	go func() { _, err := x.Read(make([]byte, 1)); done <- err }()
	// Wait until the native read deadline has been armed, then shorten it as
	// shutdown/interrupt code may do. It must return rather than retry-spin.
	c := x.(*tcpIdleConn)
	until := time.Now().Add(time.Second)
	for c.read.due.Load() == 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if c.read.due.Load() == 0 {
		t.Fatal("read not armed")
	}
	_ = x.SetReadDeadline(time.Now())
	select {
	case err := <-done:
		if e, ok := err.(net.Error); !ok || !e.Timeout() {
			t.Fatalf("expected timeout, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("external deadline ignored")
	}
}

func TestTCPIdleBlockedWriteExpires(t *testing.T) {
	a, _ := tcpPair(t)
	b, _ := tcpPair(t)
	if err := a.SetWriteBuffer(1024); err != nil {
		t.Fatal(err)
	}
	const idle = 100 * time.Millisecond
	x, _ := TCPIdleConns(a, b, idle)
	started := time.Now()
	n, err := x.Write(make([]byte, 4*1024*1024))
	if e, ok := err.(net.Error); !ok || !e.Timeout() {
		t.Fatalf("expected write timeout: %d %v", n, err)
	}
	if elapsed := time.Since(started); elapsed < idle/2 || elapsed > time.Second {
		t.Fatalf("write timeout after %s", elapsed)
	}
}

// An explicit write deadline must not make a pending read reject later shared
// activity. Exercise the read across several expired socket deadlines while
// only the other endpoint keeps the tunnel active.
func TestTCPIdleWriteDeadlineDoesNotCancelRead(t *testing.T) {
	a, ap := tcpPair(t)
	b, bp := tcpPair(t)
	const idle = 80 * time.Millisecond
	x, y := TCPIdleConns(a, b, idle)
	done := make(chan error, 1)
	go func() { _, err := x.Read(make([]byte, 1)); done <- err }()
	c := x.(*tcpIdleConn)
	until := time.Now().Add(time.Second)
	for c.read.due.Load() == 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if c.read.due.Load() == 0 {
		t.Fatal("read not armed")
	}
	if err := x.SetWriteDeadline(time.Now()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		sent := make(chan error, 1)
		go func() { _, err := bp.Write([]byte("a")); sent <- err }()
		if _, err := y.Read(make([]byte, 1)); err != nil {
			t.Fatal(err)
		}
		if err := <-sent; err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-done:
			t.Fatalf("write deadline interrupted active read: %v", err)
		default:
		}
		time.Sleep(idle / 4)
	}
	if _, err := ap.Write([]byte("r")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("read did not finish")
	}
}

func TestTCPIdleMaximumTimeoutStillArms(t *testing.T) {
	a, ap := tcpPair(t)
	b, _ := tcpPair(t)
	x, _ := TCPIdleConns(a, b, time.Duration(1<<63-1))
	done := make(chan error, 1)
	go func() { _, err := x.Read(make([]byte, 1)); done <- err }()
	c := x.(*tcpIdleConn)
	until := time.Now().Add(time.Second)
	for c.read.due.Load() == 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if c.read.due.Load() <= 0 {
		t.Fatal("maximum idle timeout failed to arm a future deadline")
	}
	if _, err := ap.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("read did not complete")
	}
}

func keepTCPIdleActive(bp net.Conn, stop <-chan struct{}, activity chan<- error, interval time.Duration) {
	for {
		select {
		case <-stop:
			activity <- nil
			return
		default:
		}
		if _, err := bp.Write([]byte("a")); err != nil {
			activity <- err
			return
		}
		time.Sleep(interval)
	}
}

func drainIdleActivity(conn net.Conn, readDone chan<- struct{}) {
	defer close(readDone)
	buf := make([]byte, 1)
	for {
		if _, err := conn.Read(buf); err != nil {
			return
		}
	}
}

func fillRetryPayload(payload []byte) {
	for i := range payload {
		payload[i] = byte(i ^ (i >> 8) ^ (i >> 16))
	}
}

func assertIdleReadStillBlocked(t *testing.T, blockedRead <-chan error) {
	t.Helper()
	select {
	case err := <-blockedRead:
		t.Fatalf("read returned during shared activity: %v", err)
	default:
	}
}
