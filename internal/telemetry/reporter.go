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

package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"strconv"
	"time"

	"github.com/soulteary/portmap/internal/stats"
)

// Reporter samples all process instances on one persistent connection. It never
// runs on the forwarding goroutines; a stalled collector only delays reporting.
type Reporter struct {
	Endpoint  string
	Providers []stats.Provider
	Events    *stats.EventLog
	Interval  time.Duration
}

func (r *Reporter) Run(ctx context.Context) {
	network, address, err := Endpoint(r.Endpoint)
	if err != nil {
		return
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return
	}
	boot := hex.EncodeToString(id[:])
	interval := r.Interval
	if interval <= 0 {
		interval = time.Second
	}
	state := reportState{backoff: time.Second}
	for ctx.Err() == nil {
		conn, err := (&net.Dialer{Timeout: ioTimeout}).DialContext(ctx, network, address)
		if err == nil {
			stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
			err = r.session(ctx, conn, boot, interval, &state)
			stop()
			_ = conn.Close()
		}
		if err == nil || ctx.Err() != nil {
			return
		}
		// Acknowledged batches reset retry delay; failed sessions back off.
		jitter := time.Duration(randByte()) * state.backoff / 1024
		if !wait(ctx, state.backoff+jitter) {
			return
		}
		state.backoff = min(state.backoff*2, 30*time.Second)
	}
}

func randByte() byte {
	var b [1]byte
	_, _ = rand.Read(b[:])
	return b[0]
}

func wait(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (r *Reporter) session(ctx context.Context, conn net.Conn, boot string, interval time.Duration, state *reportState) error {
	if err := conn.SetDeadline(time.Now().Add(ioTimeout)); err != nil {
		return err
	}
	if err := writeFrame(conn, hello{Version, boot}); err != nil {
		return err
	}
	var a ack
	if err := readFrame(conn, &a); err != nil {
		return err
	}
	if a.Version != Version {
		return net.ErrClosed
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		state.seq++
		b := batch{Version: Version, BootID: boot, Seq: state.seq, SampledAt: time.Now().UTC()}
		next := r.fillBatch(&b, state.cursor, state.dropped)
		if err := conn.SetDeadline(time.Now().Add(ioTimeout)); err != nil {
			return err
		}
		if err := writeFrame(conn, b); err != nil {
			return err
		}
		if err := readFrame(conn, &a); err != nil {
			return err
		}
		if a.Version != Version || a.Seq != b.Seq {
			return net.ErrClosed
		}
		state.cursor, state.dropped = next, b.EventsDropped
		state.backoff = time.Second
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
	return nil
}

func bounded(s string) string {
	if len(s) > 256 {
		return s[:256]
	}
	return s
}

func (r *Reporter) fillBatch(b *batch, cursor, dropped uint64) uint64 {
	for i, p := range r.Providers {
		sample := Sample{ID: strconv.Itoa(i + 1), Snapshot: p.Snapshot()}
		if named, ok := p.(interface{ StatsLabel() string }); ok {
			sample.Label = bounded(named.StatsLabel())
		}
		b.Samples = append(b.Samples, sample)
	}
	events, next, lost := r.Events.ReadSince(cursor)
	if len(events) > 1000 {
		extra := len(events) - 1000
		lost += uint64(extra)
		events = events[extra:]
	}
	for i := range events {
		events[i].Client = bounded(events[i].Client)
		events[i].Target = bounded(events[i].Target)
	}
	b.Events, b.EventsDropped, b.EventsCursor = events, dropped+lost, next
	return next
}

type reportState struct {
	seq, cursor, dropped uint64
	backoff              time.Duration
}
