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

package stats

import (
	"sync"
	"testing"
)

func TestEventLogCapacityAndOrder(t *testing.T) {
	const max = 5
	l := NewEventLog(max)
	// 追加 12 条，超过容量上限。
	for i := int64(0); i < 12; i++ {
		l.Append(Event{ConnID: i})
	}

	snap := l.Snapshot()
	if len(snap) != max {
		t.Fatalf("Snapshot len=%d, want %d", len(snap), max)
	}
	// 应仅保留最后 max 条（ConnID 7..11），且保持追加顺序。
	for i, e := range snap {
		want := int64(12 - max + i)
		if e.ConnID != want {
			t.Errorf("snap[%d].ConnID=%d, want %d", i, e.ConnID, want)
		}
	}
}

func TestEventLogSnapshotIsolation(t *testing.T) {
	l := NewEventLog(10)
	l.Append(Event{ConnID: 1, Kind: "open"})
	l.Append(Event{ConnID: 2, Kind: "close"})

	first := l.Snapshot()
	// 修改返回的副本不应影响内部存储。
	first[0].ConnID = 999
	first[0].Kind = "mutated"

	second := l.Snapshot()
	if second[0].ConnID != 1 || second[0].Kind != "open" {
		t.Errorf("internal state mutated via returned slice: %+v", second[0])
	}
}

func TestEventLogConcurrentAppend(t *testing.T) {
	const (
		workers = 50
		iters   = 1000
		total   = workers * iters
	)
	// 容量足够大，确保所有事件都被保留，便于校验计数。
	l := NewEventLog(total)

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iters; i++ {
				l.Append(Event{Kind: "open"})
			}
		}()
	}
	wg.Wait()

	if got := len(l.Snapshot()); got != total {
		t.Errorf("Snapshot len=%d, want %d", got, total)
	}
}

func TestEventLogAppendNilSafe(t *testing.T) {
	var l *EventLog // nil 接收者
	// 不应 panic。
	l.Append(Event{ConnID: 1, Kind: "open"})
}

func TestNewEventLogDefaultCapacity(t *testing.T) {
	l := NewEventLog(0)
	// 默认容量为 1000：追加 1500 条后应仅保留最后 1000 条。
	for i := int64(0); i < 1500; i++ {
		l.Append(Event{ConnID: i})
	}
	snap := l.Snapshot()
	if len(snap) != 1000 {
		t.Fatalf("Snapshot len=%d, want 1000", len(snap))
	}
	if snap[0].ConnID != 500 {
		t.Errorf("oldest kept ConnID=%d, want 500", snap[0].ConnID)
	}
	if snap[len(snap)-1].ConnID != 1499 {
		t.Errorf("newest ConnID=%d, want 1499", snap[len(snap)-1].ConnID)
	}
}
