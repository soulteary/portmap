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

// Package stats 提供 forward 与 proxy 共用的运行时统计计数器：连接数
// （活跃/累计/拒绝）、拨号失败数、上/下行字节数与运行时长。所有计数均基于
// atomic.Int64，可在多 goroutine 下无锁并发累加；Snapshot 返回某一时刻的
// 一致视图，供 SIGUSR1 日志、JSON 与 Prometheus 端点共用。
package stats

import (
	"sync/atomic"
	"time"
)

// Counters 汇聚一个服务实例的运行时统计。零值不可直接使用，请经 New 构造以
// 记录 StartTime。所有字段用 atomic.Int64 保证并发安全。
type Counters struct {
	activeConns   atomic.Int64
	totalConns    atomic.Int64
	rejectedConns atomic.Int64
	dialErrors    atomic.Int64
	upBytes       atomic.Int64
	downBytes     atomic.Int64

	startTime time.Time
}

// New 创建一个 Counters，并把 StartTime 记为当前时间，用于计算 uptime。
func New() *Counters {
	return &Counters{startTime: time.Now()}
}

// ConnOpened 记录一个新连接：活跃数 +1，累计数 +1。
func (c *Counters) ConnOpened() {
	c.activeConns.Add(1)
	c.totalConns.Add(1)
}

// ConnClosed 记录一个连接结束：活跃数 -1。
func (c *Counters) ConnClosed() {
	c.activeConns.Add(-1)
}

// AddUp 累加上行（客户端 -> 目标）字节数。
func (c *Counters) AddUp(n int64) {
	if n != 0 {
		c.upBytes.Add(n)
	}
}

// AddDown 累加下行（目标 -> 客户端）字节数。
func (c *Counters) AddDown(n int64) {
	if n != 0 {
		c.downBytes.Add(n)
	}
}

// Reject 记录一次因限流（MaxConns）被拒绝的连接。
func (c *Counters) Reject() {
	c.rejectedConns.Add(1)
}

// DialError 记录一次出站拨号失败。
func (c *Counters) DialError() {
	c.dialErrors.Add(1)
}

// ActiveConns 返回当前活跃连接数。
func (c *Counters) ActiveConns() int64 { return c.activeConns.Load() }

// TotalConns 返回累计处理的连接数。
func (c *Counters) TotalConns() int64 { return c.totalConns.Load() }

// Snapshot 返回某一时刻的计数快照。各字段独立读取，故并非严格原子的整体
// 一致视图，但对监控用途足够（各计数本身仍是原子读取）。
func (c *Counters) Snapshot() Snapshot {
	return Snapshot{
		ActiveConns:   c.activeConns.Load(),
		TotalConns:    c.totalConns.Load(),
		RejectedConns: c.rejectedConns.Load(),
		DialErrors:    c.dialErrors.Load(),
		UpBytes:       c.upBytes.Load(),
		DownBytes:     c.downBytes.Load(),
		Uptime:        time.Since(c.startTime),
	}
}

// Snapshot 是一个可序列化的统计快照，供日志、JSON 与 Prometheus 共用。
type Snapshot struct {
	ActiveConns   int64         `json:"active_conns"`
	TotalConns    int64         `json:"total_conns"`
	RejectedConns int64         `json:"rejected_conns"`
	DialErrors    int64         `json:"dial_errors"`
	UpBytes       int64         `json:"up_bytes"`
	DownBytes     int64         `json:"down_bytes"`
	Uptime        time.Duration `json:"uptime_ns"`
}

// Add 把另一个快照的计数并入当前快照，用于多实例聚合。Uptime 取两者较大值，
// 以反映整个进程的运行时长。
func (s Snapshot) Add(o Snapshot) Snapshot {
	s.ActiveConns += o.ActiveConns
	s.TotalConns += o.TotalConns
	s.RejectedConns += o.RejectedConns
	s.DialErrors += o.DialErrors
	s.UpBytes += o.UpBytes
	s.DownBytes += o.DownBytes
	if o.Uptime > s.Uptime {
		s.Uptime = o.Uptime
	}
	return s
}
