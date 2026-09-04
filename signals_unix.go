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

//go:build unix

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/stats"
)

// statusProvider 是能提供统计快照的服务（forward.Server / proxy.Server）。
type statusProvider interface {
	Snapshot() stats.Snapshot
}

// watchStatusSignal 在类 Unix 平台监听 SIGUSR1，收到后打印统计快照
// （活跃/累计/拒绝连接、拨号失败、上下行字节、运行时长）。ctx 取消时停止监听。
func watchStatusSignal(ctx context.Context, srv statusProvider) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		defer signal.Stop(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				snap := srv.Snapshot()
				log.Printf(i18n.T(i18n.KeyLogStatusFull),
					snap.ActiveConns, snap.TotalConns, snap.RejectedConns,
					snap.DialErrors, snap.UpBytes, snap.DownBytes,
					snap.Uptime.Round(time.Second))
			}
		}
	}()
}
