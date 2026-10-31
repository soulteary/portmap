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
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// TCPIdleConns wraps a native TCP tunnel with a shared idle timeout. As with
// IdleConn, starting I/O and completing nonempty I/O both refresh activity.
// Socket deadlines are extended lazily: an old deadline may wake an operation,
// but is only reported as a timeout if the whole tunnel is actually idle.
// This requires TCP's retryable deadlines; SSH channels that close on deadline
// expiry must continue to use IdleConn instead.
func TCPIdleConns(a, b *net.TCPConn, timeout time.Duration) (net.Conn, net.Conn) {
	if timeout <= 0 {
		return a, b
	}
	g := &tcpIdleState{start: time.Now(), timeout: timeout}
	return &tcpIdleConn{Conn: a, group: g}, &tcpIdleConn{Conn: b, group: g}
}

type tcpIdleState struct {
	start   time.Time
	timeout time.Duration
	last    atomic.Int64 // monotonic duration since start; never moves backwards
}

func (g *tcpIdleState) touch() {
	now := int64(time.Since(g.start))
	for old := g.last.Load(); now > old; old = g.last.Load() {
		if g.last.CompareAndSwap(old, now) {
			return
		}
	}
}

// deadline saturates relative time arithmetic so even the largest accepted
// positive time.Duration arms a future deadline instead of wrapping negative.
func (g *tcpIdleState) deadline() int64 {
	const maxDeadline int64 = 1<<63 - 1
	last := g.last.Load()
	if last > maxDeadline-int64(g.timeout) {
		return maxDeadline
	}
	return last + int64(g.timeout)
}

// Each direction owns its installed deadline, so extending a read deadline
// never overwrites a concurrent write deadline. Locks are only taken when a
// socket deadline needs installing, not on every packet.
type tcpIdleDeadline struct {
	mu  sync.Mutex
	due atomic.Int64
}

type tcpIdleConn struct {
	net.Conn    // Do not expose TCPConn.ReadFrom/WriteTo: they bypass activity tracking.
	group       *tcpIdleState
	read, write tcpIdleDeadline
}

func (c *tcpIdleConn) arm(d *tcpIdleDeadline, set func(time.Time) error, expired bool) error {
	// The caller has just touched activity, so last is also a monotonic clock
	// sample. Reuse it rather than reading the clock again on the hot path.
	if !expired && d.due.Load() > c.group.last.Load() {
		return nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	due := c.group.deadline()
	if !expired && d.due.Load() > c.group.last.Load() {
		return nil
	}
	if d.due.Load() >= due {
		return nil
	}
	if err := set(c.group.start.Add(time.Duration(due))); err != nil {
		return err
	}
	d.due.Store(due)
	return nil
}

func (c *tcpIdleConn) retry(err error, d *tcpIdleDeadline) bool {
	e, ok := err.(net.Error)
	if !ok || !e.Timeout() {
		return false
	}
	now := int64(time.Since(c.group.start))
	// A deadline explicitly shortened by the caller must not turn into a busy
	// retry loop. Only our own expired socket deadlines are eligible for retry.
	return now >= d.due.Load() && now < c.group.deadline()
}

func (c *tcpIdleConn) Read(p []byte) (int, error) {
	c.group.touch()
	expired := false
	for {
		if err := c.arm(&c.read, c.Conn.SetReadDeadline, expired); err != nil {
			return 0, err
		}
		n, err := c.Conn.Read(p)
		if n > 0 {
			c.group.touch()
		}
		if n != 0 || !c.retry(err, &c.read) {
			return n, err
		}
		expired = true
	}
}

func (c *tcpIdleConn) Write(p []byte) (int, error) {
	c.group.touch()
	total := 0
	expired := false
	for {
		if err := c.arm(&c.write, c.Conn.SetWriteDeadline, expired); err != nil {
			return total, err
		}
		n, err := c.Conn.Write(p)
		total += n
		// TCP writes may return partial data with a stale timeout. Retry only
		// the unwritten suffix, and decide before touch so partial progress cannot
		// revive a genuinely idle tunnel.
		retry := c.retry(err, &c.write)
		if n > 0 {
			c.group.touch()
		}
		if !retry {
			return total, err
		}
		expired = true
		p = p[n:]
		if len(p) == 0 {
			return total, nil
		}
	}
}

func (c *tcpIdleConn) CloseWrite() error {
	return c.Conn.(*net.TCPConn).CloseWrite()
}
