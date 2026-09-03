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

import "sync"

// Event 表示一条连接事件，可 JSON 序列化，供 Web 面板展示最近的连接活动。
// 与 atomic 计数器不同，Event 记录单次连接的明细（客户端、目标、字节数、
// 时长等），用于事件流/日志视图。
type Event struct {
	// Time 事件发生时间，格式化为 "2006-01-02 15:04:05" 便于阅读。
	Time string `json:"time"`
	// Kind 事件类型，取值 "open"、"close"、"reject"、"dial-error" 之一。
	Kind string `json:"kind"`
	// Proto 传输/代理协议，例如 "tcp"、"udp"、"socks5"、"http"。
	Proto string `json:"proto"`
	// Client 远端客户端地址。
	Client string `json:"client"`
	// Target 目标/目的地址。
	Target string `json:"target"`
	// UpBytes 上行（客户端 -> 目标）字节数。
	UpBytes int64 `json:"up_bytes"`
	// DownBytes 下行（目标 -> 客户端）字节数。
	DownBytes int64 `json:"down_bytes"`
	// DurationMs 连接时长，单位毫秒。
	DurationMs int64 `json:"duration_ms"`
	// ConnID 连接编号，用于关联同一连接的多条事件。
	ConnID int64 `json:"conn_id"`
}

// EventLog 是线程安全的环形缓冲区，保留最近 max 条连接事件。内部以切片存储，
// 通过 sync.RWMutex 保护并发读写；当条数超过 max 时丢弃最旧的记录，仅保留
// 最新的 max 条。适合为 Web 面板提供“最近事件”这类有界内存的展示数据。
type EventLog struct {
	mu      sync.RWMutex
	entries []Event
	max     int
}

// NewEventLog 创建一个容量上限为 max 的事件缓冲区。max <= 0 时默认取 1000。
func NewEventLog(max int) *EventLog {
	if max <= 0 {
		max = 1000
	}
	return &EventLog{entries: make([]Event, 0, max), max: max}
}

// Append 追加一条事件；若总数超过 max，则裁剪最旧的记录，仅保留最新的 max 条。
// 对 nil *EventLog 调用是安全的空操作，因此调用方无需在插桩处判空即可直接调用。
func (l *EventLog) Append(e Event) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, e)
	if len(l.entries) > l.max {
		l.entries = l.entries[len(l.entries)-l.max:]
	}
}

// Snapshot 返回当前事件的副本切片，与内部存储相互隔离，调用方对返回值的修改
// 不会影响内部状态（反之亦然）。
func (l *EventLog) Snapshot() []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Event, len(l.entries))
	copy(out, l.entries)
	return out
}
