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
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/portmap/internal/stats"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenStore(filepath.Join(t.TempDir(), "stats.journal"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}
func sampleBatch(seq uint64, up int64) batch {
	return batch{Version: Version, BootID: strings.Repeat("a", 32), Seq: seq, SampledAt: time.Now(), Samples: []Sample{{ID: "1", Snapshot: stats.Snapshot{UpBytes: up, ActiveConns: 1}}}}
}
func eventually(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition did not become true")
}
func TestEndpointAndFrames(t *testing.T) {
	for _, raw := range []string{"unix:///tmp/stats.sock", "tcp://127.0.0.1:9190", "tcp://[::1]:9190"} {
		if _, _, err := Endpoint(raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"", "http://localhost:9190", "unix://relative", "tcp://:9190", "tcp://localhost:9190/path", "tcp://user@localhost:9190", "unix:///tmp/a?x=y"} {
		if _, _, err := Endpoint(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	var buf bytes.Buffer
	want := ack{Version, 42}
	if err := writeFrame(&buf, want); err != nil {
		t.Fatal(err)
	}
	var got ack
	if err := readFrame(&buf, &got); err != nil || got != want {
		t.Fatalf("%+v %v", got, err)
	}
	for _, size := range []uint32{0, maxFrame + 1} {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], size)
		if readFrame(bytes.NewReader(b[:]), &got) == nil {
			t.Fatal("accepted bad size")
		}
	}
	if writeFrame(io.Discard, strings.Repeat("x", maxFrame+1)) == nil {
		t.Fatal("accepted oversized payload")
	}
	if writeFrame(io.Discard, make(chan int)) == nil {
		t.Fatal("accepted invalid JSON value")
	}
	if readFrame(bytes.NewReader([]byte{0}), &got) == nil {
		t.Fatal("accepted truncated header")
	}
}
func TestStoreReplayAndDeduplication(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stats.journal")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	b := sampleBatch(1, 10)
	b.Events = []stats.Event{{Kind: "open", ConnID: 1}}
	b.EventsCursor = 1
	if err := s.accept(b); err != nil {
		t.Fatal(err)
	}
	if err := s.accept(b); err != nil {
		t.Fatal(err)
	}
	if len(s.Events.Snapshot()) != 1 || len(s.Records(true)) != 1 {
		t.Fatal("duplicate batch")
	}
	if _, err := OpenStore(path); err == nil {
		t.Fatal("journal lock not enforced")
	}
	_ = s.Close()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte{0, 0})
	_ = f.Close()
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	b = sampleBatch(3, 30) // missing sequence 2 must not lose cumulative bytes
	b.Events = []stats.Event{{Kind: "open", ConnID: 1}, {Kind: "close", ConnID: 1}}
	b.EventsCursor = 2
	if err := s.accept(b); err != nil {
		t.Fatal(err)
	}
	if got := s.Snapshot(); got.UpBytes != 30 {
		t.Fatalf("%+v", got)
	}
	if len(s.Events.Snapshot()) != 2 {
		t.Fatal("events replayed twice")
	}
	if err := s.accept(sampleBatch(4, 2)); err == nil {
		t.Fatal("accepted decreasing counters")
	}
	b = sampleBatch(1, 5)
	b.BootID = strings.Repeat("b", 32)
	if err := s.accept(b); err != nil {
		t.Fatal(err)
	}
	if s.Snapshot().UpBytes != 35 {
		t.Fatal("restart not separated")
	}
	assertStaleTotals(t, s, 35)
}
func TestReporterCollectorReconnect(t *testing.T) {
	for _, network := range []string{"unix", "tcp"} {
		t.Run(network, func(t *testing.T) {
			store := openTestStore(t)
			dir, err := os.MkdirTemp("", "pmstats")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = os.RemoveAll(dir) }()
			endpoint := "unix://" + filepath.Join(dir, "s.sock")
			if network == "tcp" {
				endpoint = "tcp://127.0.0.1:0"
			}
			ln, err := Listen(endpoint)
			if err != nil {
				t.Fatal(err)
			}
			if network == "tcp" {
				endpoint = "tcp://" + ln.Addr().String()
			}
			_ = ln.Close() // reporter starts before the collector
			c := stats.New()
			c.AddUp(7)
			events := stats.NewEventLog(2)
			for range 4 {
				events.Append(stats.Event{Kind: "open"})
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reporterDone := make(chan struct{})
			go func() {
				defer close(reporterDone)
				(&Reporter{Endpoint: endpoint, Providers: []stats.Provider{c}, Events: events, Interval: 10 * time.Millisecond}).Run(ctx)
			}()
			time.Sleep(30 * time.Millisecond)
			start := func() (context.CancelFunc, <-chan error) {
				ln, err := Listen(endpoint)
				if err != nil {
					t.Fatal(err)
				}
				child, stop := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() { done <- Serve(child, ln, store) }()
				return stop, done
			}
			stop, done := start()
			eventually(t, func() bool { return store.Snapshot().UpBytes == 7 })
			if store.Records(false)[0].Batch.EventsDropped != 2 {
				t.Fatal("missing dropped count")
			}
			stop()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			c.AddUp(11)
			c.AddDown(13)
			events.Append(stats.Event{Kind: "close"})
			stop, done = start()
			eventually(t, func() bool { s := store.Snapshot(); return s.UpBytes == 18 && s.DownBytes == 13 })
			cancel()
			select {
			case <-reporterDone:
			case <-time.After(time.Second):
				t.Fatal("reporter stuck")
			}
			stop()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			if len(store.Events.Snapshot()) != 3 {
				t.Fatal("duplicate or missing events")
			}
		})
	}
}
func TestCollectorRejectsInvalidHandshakeAndCancelsStall(t *testing.T) {
	s := openTestStore(t)
	a, b := net.Pipe()
	done := make(chan error, 1)
	go func() { defer func() { _ = a.Close() }(); done <- receive(a, s) }()
	if err := writeFrame(b, hello{Version: 99, BootID: strings.Repeat("a", 32)}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil {
		t.Fatal("accepted version")
	}
	_ = b.Close()
	ctx, cancel := context.WithCancel(context.Background())
	ln, err := Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { done <- Serve(ctx, ln, s) }()
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("collector stuck")
	}
	if _, err := Listen("tcp://0.0.0.0:0"); err == nil {
		t.Fatal("public listener accepted")
	}
}
func TestReporterCancellationWithSilentPeer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	accepted := make(chan net.Conn, 1)
	go func() { c, _ := ln.Accept(); accepted <- c }()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		(&Reporter{Endpoint: "tcp://" + ln.Addr().String(), Providers: []stats.Provider{stats.New()}}).Run(ctx)
	}()
	conn := <-accepted
	defer func() { _ = conn.Close() }()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reporter blocked on peer")
	}
}

func TestValidationAndJournalFailures(t *testing.T) {
	s := openTestStore(t)
	for _, mutate := range []func(*batch){
		func(b *batch) { b.Version = 99 }, func(b *batch) { b.Seq = 0 }, func(b *batch) { b.Samples = nil },
		func(b *batch) { b.Samples[0].UpBytes = -1 }, func(b *batch) { b.Samples[0].ID = "" },
		func(b *batch) { b.Samples = append(b.Samples, b.Samples[0]) },
	} {
		b := sampleBatch(1, 5)
		mutate(&b)
		if s.accept(b) == nil {
			t.Fatalf("accepted %+v", b)
		}
	}
	b := sampleBatch(1, 5)
	b.EventsCursor = 2
	b.EventsDropped = 2
	if err := s.accept(b); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*batch){func(b *batch) { b.EventsCursor = 1 }, func(b *batch) { b.EventsDropped = 1 }, func(b *batch) { b.Samples = append(b.Samples, Sample{ID: "2"}) }, func(b *batch) { b.Samples[0].ID = "other" }} {
		next := sampleBatch(2, 6)
		next.EventsCursor = 2
		next.EventsDropped = 2
		mutate(&next)
		if s.accept(next) == nil {
			t.Fatalf("accepted %+v", next)
		}
	}
	full := &Store{processes: make(map[string]Record)}
	for i := range maxProcesses {
		full.processes[strconv.Itoa(i)] = Record{}
	}
	if full.validate(sampleBatch(1, 1)) == nil {
		t.Fatal("accepted too many processes")
	}
	if _, err := OpenStore(filepath.Join(t.TempDir(), "missing", "journal")); err == nil {
		t.Fatal("opened missing parent")
	}
	for _, payload := range []any{"not a record", Record{Batch: sampleBatch(0, 0)}} {
		path := filepath.Join(t.TempDir(), "corrupt")
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := writeFrame(f, payload); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		if _, err := OpenStore(path); err == nil {
			t.Fatal("accepted corrupt journal")
		}
	}
	_ = s.file.Close()
	if s.accept(sampleBatch(2, 6)) == nil {
		t.Fatal("accepted closed journal")
	}
}

func TestEndpointFailures(t *testing.T) {
	if _, _, err := Endpoint("tcp://[bad"); err == nil {
		t.Fatal("accepted malformed URL")
	}
	if _, err := Listen("bad"); err == nil {
		t.Fatal("accepted invalid listener")
	}
	ln, err := ListenWithPublic("tcp://0.0.0.0:0", true)
	if err != nil {
		t.Fatal(err)
	}
	_ = ln.Close()
	if _, err := Listen("unix:///nonexistent/portmap/sock"); err == nil {
		t.Fatal("accepted missing directory")
	}
	if bounded(strings.Repeat("x", 1000)) != strings.Repeat("x", 256) {
		t.Fatal("unbounded metadata")
	}
	(&Reporter{Endpoint: "bad"}).Run(context.Background())
}

type labeledCounter struct{ *stats.Counters }

func (l labeledCounter) StatsLabel() string { return "forward tcp://127.0.0.1:1234" }
func TestBatchBounds(t *testing.T) {
	events := stats.NewEventLog(2000)
	for range 1500 {
		events.Append(stats.Event{Client: strings.Repeat("x", 1000), Target: strings.Repeat("y", 1000)})
	}
	r := Reporter{Providers: []stats.Provider{labeledCounter{stats.New()}}, Events: events}
	b := batch{}
	next := r.fillBatch(&b, 0, 0)
	if next != 1500 || b.EventsDropped != 500 || len(b.Events) != 1000 || len(b.Events[0].Client) != 256 || b.Samples[0].Label == "" {
		t.Fatalf("invalid bounded batch")
	}
	if err := writeFrame(io.Discard, b); err != nil {
		t.Fatal(err)
	}
	s := openTestStore(t)
	for i := range 1001 {
		b := sampleBatch(uint64(i+1), int64(i))
		s.install(Record{Batch: b})
	}
	if len(s.Records(true)) != 1000 {
		t.Fatal("history not bounded")
	}
}

func TestReceiveRejectsProcessChange(t *testing.T) {
	s := openTestStore(t)
	for _, change := range []bool{false, true} {
		a, b := net.Pipe()
		done := make(chan error, 1)
		go func() { defer func() { _ = a.Close() }(); done <- receive(a, s) }()
		h := hello{Version, strings.Repeat("a", 32)}
		if err := writeFrame(b, h); err != nil {
			t.Fatal(err)
		}
		var ack ack
		if err := readFrame(b, &ack); err != nil {
			t.Fatal(err)
		}
		payload := sampleBatch(1, 2)
		if change {
			payload.BootID = strings.Repeat("b", 32)
		} else {
			payload.Samples[0].UpBytes = -1
		}
		if err := writeFrame(b, payload); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err == nil {
			t.Fatal("accepted invalid payload")
		}
		_ = b.Close()
	}
}

// Broken streams must fail reporting without committing a local event cursor.
func TestReporterRejectsBadAcknowledgements(t *testing.T) {
	for _, scenario := range []string{"bad-version", "bad-sequence", "closed"} {
		t.Run(scenario, func(t *testing.T) {
			a, b := net.Pipe()
			defer func() { _ = a.Close() }()
			peerDone := make(chan struct{})
			go func() {
				defer close(peerDone)
				defer func() { _ = b.Close() }()
				var h hello
				if readFrame(b, &h) != nil {
					return
				}
				if scenario == "closed" {
					return
				}
				version := Version
				if scenario == "bad-version" {
					version++
				}
				if writeFrame(b, ack{Version: version}) != nil || version != Version {
					return
				}
				var payload batch
				if readFrame(b, &payload) != nil {
					return
				}
				_ = writeFrame(b, ack{Version: Version, Seq: payload.Seq + 1})
			}()
			state := reportState{backoff: time.Second}
			r := Reporter{Providers: []stats.Provider{stats.New()}}
			if r.session(context.Background(), a, strings.Repeat("a", 32), time.Second, &state) == nil {
				t.Fatal("accepted bad acknowledgement")
			}
			if state.cursor != 0 {
				t.Fatal("advanced unacknowledged cursor")
			}
			<-peerDone
		})
	}
}

type failedDeadlineConn struct{ net.Conn }

func (failedDeadlineConn) SetDeadline(time.Time) error { return io.ErrClosedPipe }
func TestDeadlineAndWriteFailures(t *testing.T) {
	s := openTestStore(t)
	if receive(failedDeadlineConn{}, s) == nil {
		t.Fatal("ignored deadline failure")
	}
	state := reportState{}
	if (&Reporter{}).session(context.Background(), failedDeadlineConn{}, "", time.Second, &state) == nil {
		t.Fatal("ignored deadline failure")
	}
	a, b := net.Pipe()
	_ = b.Close()
	if (&Reporter{}).session(context.Background(), a, "", time.Second, &state) == nil {
		t.Fatal("ignored write failure")
	}
	_ = a.Close()
	path := filepath.Join(t.TempDir(), "readonly")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	readonly := &Store{file: f, processes: make(map[string]Record), Events: stats.NewEventLog(1)}
	if readonly.accept(sampleBatch(1, 5)) == nil {
		t.Fatal("acknowledged unwritable journal")
	}
	if readonly.Snapshot().UpBytes != 0 {
		t.Fatal("installed unpersisted counters")
	}
	var out ack
	if readFrame(bytes.NewReader([]byte{0, 0, 0, 2, '{'}), &out) == nil {
		t.Fatal("accepted partial body")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	(&Reporter{Endpoint: "tcp://127.0.0.1:1"}).Run(ctx)
}

func TestReceiverConnectionLimit(t *testing.T) {
	s := openTestStore(t)
	ln, err := Listen("tcp://127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, s) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	clients := make([]net.Conn, 0, 128)
	defer func() {
		for _, c := range clients {
			_ = c.Close()
		}
	}()
	for range 128 {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, conn)
		if err := writeFrame(conn, hello{Version, strings.Repeat("a", 32)}); err != nil {
			t.Fatal(err)
		}
		var response ack
		if err := readFrame(conn, &response); err != nil {
			t.Fatal(err)
		}
	}
	extra, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = extra.Close() }()
	_ = extra.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := extra.Read(make([]byte, 1)); err == nil {
		t.Fatal("excess connection not closed")
	}
}

func assertStaleTotals(t *testing.T, s *Store, want int64) {
	t.Helper()

	s.mu.Lock()
	for id, r := range s.processes {
		r.ReceivedAt = time.Now().Add(-time.Minute)
		s.processes[id] = r
	}
	s.mu.Unlock()
	if got := s.Snapshot(); got.UpBytes != want || got.ActiveConns != 0 {
		t.Fatalf("stale %+v", got)
	}
}
