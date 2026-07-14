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

//go:build !windows

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/soulteary/portmap/internal/forward"
)

// watchStatusSignal 在类 Unix 平台监听 SIGUSR1，收到后打印活跃/累计连接快照。
// ctx 取消时停止监听。Windows 无 SIGUSR1，由同名 no-op 版本替代。
func watchStatusSignal(ctx context.Context, srv *forward.Server) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		defer signal.Stop(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				log.Printf("status: active=%d total=%d", srv.ActiveConns(), srv.TotalConns())
			}
		}
	}()
}
