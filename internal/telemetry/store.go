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
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/soulteary/portmap/internal/stats"
	"golang.org/x/sys/unix"
)

const maxProcesses = 10000

// Record is a durable cumulative sample. ReceivedAt is collector time, not a
// claim about when bytes were transferred during a reporting outage.
type Record struct {
	ReceivedAt time.Time `json:"received_at"`
	Batch      batch     `json:"batch"`
}

// Store persists accepted batches before acknowledging them. The journal is
// replayed on restart, so reconnects cannot count the same process twice.
type Store struct {
	mu        sync.RWMutex
	file      *os.File
	processes map[string]Record
	history   []Record
	Events    *stats.EventLog
}

func OpenStore(path string) (*Store, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, err
	}
	s := &Store{file: f, processes: make(map[string]Record), Events: stats.NewEventLog(1000)}
	if err := s.replay(); err != nil {
		_ = f.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) replay() error {
	var offset int64
	for {
		var record Record
		err := readFrame(s.file, &record)
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			// A crash may leave one incomplete final frame. Never accept it as a baseline.
			if err := s.file.Truncate(offset); err != nil {
				return err
			}
			_, err = s.file.Seek(offset, io.SeekStart)
			return err
		}
		if err != nil {
			return err
		}
		if err := s.validate(record.Batch); err != nil {
			return err
		}
		s.install(record)
		offset, err = s.file.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
	}
}

func (s *Store) Close() error { return s.file.Close() }

func (s *Store) validate(b batch) error {
	if !validBatch(b) {
		return fmt.Errorf("invalid stats batch")
	}
	old, exists := s.processes[b.BootID]
	if !exists && len(s.processes) >= maxProcesses {
		return fmt.Errorf("stats process limit reached")
	}
	if exists && b.Seq <= old.Batch.Seq {
		return nil
	}
	if exists && (b.EventsCursor < old.Batch.EventsCursor || b.EventsDropped < old.Batch.EventsDropped) {
		return fmt.Errorf("stats event counters decreased")
	}
	if exists && len(b.Samples) != len(old.Batch.Samples) {
		return fmt.Errorf("stats instances changed")
	}
	return validateSamples(b.Samples, old.Batch.Samples, exists)
}

func validSnapshot(s stats.Snapshot) bool {
	return s.UpBytes >= 0 && s.DownBytes >= 0 && s.ActiveConns >= 0 && s.TotalConns >= 0 && s.RejectedConns >= 0 && s.DialErrors >= 0 && s.Uptime >= 0
}
func decreased(a, b stats.Snapshot) bool {
	return a.UpBytes < b.UpBytes || a.DownBytes < b.DownBytes || a.TotalConns < b.TotalConns || a.RejectedConns < b.RejectedConns || a.DialErrors < b.DialErrors
}

func (s *Store) accept(b batch) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.validate(b); err != nil {
		return err
	}
	if old, ok := s.processes[b.BootID]; ok && b.Seq <= old.Batch.Seq {
		return nil
	}
	record := Record{ReceivedAt: time.Now().UTC(), Batch: b}
	offset, err := s.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if err = writeFrame(s.file, record); err == nil {
		err = s.file.Sync()
	}
	if err != nil {
		_ = s.file.Truncate(offset)
		_, _ = s.file.Seek(offset, io.SeekStart)
		return err
	}
	s.install(record)
	return nil
}

func (s *Store) install(r Record) {
	old := s.processes[r.Batch.BootID]
	first := r.Batch.EventsCursor - uint64(len(r.Batch.Events))
	for i, event := range r.Batch.Events {
		if first+uint64(i)+1 > old.Batch.EventsCursor {
			event.Source = r.Batch.BootID
			s.Events.Append(event)
		}
	}
	// Keep event payloads only in the journal and the bounded shared event log.
	r.Batch.Events = nil
	s.processes[r.Batch.BootID] = r
	s.history = append(s.history, r)
	if len(s.history) > 1000 {
		s.history = s.history[len(s.history)-1000:]
	}
}

// Snapshot retains lifetime totals but excludes stale active-connection gauges.
func (s *Store) Snapshot() stats.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var total stats.Snapshot
	for _, record := range s.processes {
		for _, sample := range record.Batch.Samples {
			snap := sample.Snapshot
			if time.Since(record.ReceivedAt) > 3*time.Second {
				snap.ActiveConns = 0
			}
			total = total.Add(snap)
		}
	}
	return total
}

// Records returns either current process samples or the last 1000 batches.
func (s *Store) Records(history bool) []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, 0)
	if history {
		return append(out, s.history...)
	}
	for _, record := range s.processes {
		out = append(out, record)
	}
	return out
}

func validBatch(b batch) bool {
	return b.Version == Version && len(b.BootID) == 32 && b.Seq > 0 && len(b.Samples) > 0 && len(b.Samples) <= 1024 && len(b.Events) <= 1000 && b.EventsCursor >= uint64(len(b.Events))
}

func validateSamples(samples, previous []Sample, exists bool) error {
	seen := make(map[string]bool, len(samples))
	for i, sample := range samples {
		if sample.ID == "" || len(sample.ID) > 128 || seen[sample.ID] || !validSnapshot(sample.Snapshot) {
			return fmt.Errorf("invalid stats sample")
		}
		seen[sample.ID] = true
		if exists && (sample.ID != previous[i].ID || decreased(sample.Snapshot, previous[i].Snapshot)) {
			return fmt.Errorf("stats counters decreased")
		}
	}
	return nil
}
