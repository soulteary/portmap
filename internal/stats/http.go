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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
)

// Provider 抽象一个可提供统计快照的服务实例（forward.Server / proxy.Server 均
// 实现了 Snapshot() stats.Snapshot）。
type Provider interface {
	Snapshot() Snapshot
}

// Aggregate 聚合多个 Provider 的快照为单个 Snapshot：各计数求和，Uptime 取最大。
func Aggregate(providers []Provider) Snapshot {
	var agg Snapshot
	for _, p := range providers {
		if p == nil {
			continue
		}
		agg = agg.Add(p.Snapshot())
	}
	return agg
}

// isLoopbackHost 判断监听地址的 host 是否为回环地址。空 host（如 ":9090"）视为
// 非回环（等价于所有网卡）。
func isLoopbackHost(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "" {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// 主机名（如 "localhost"）：保守起见按非回环处理，除非解析全部为回环。
		ips, lookupErr := net.LookupIP(host)
		if lookupErr != nil || len(ips) == 0 {
			return false
		}
		for _, resolved := range ips {
			if !resolved.IsLoopback() {
				return false
			}
		}
		return true
	}
	return ip.IsLoopback()
}

// Server 是一个可选的、默认关闭、默认仅回环的 HTTP 统计端点。
// 暴露 /stats（JSON）与 /metrics（Prometheus 文本格式）两个只读端点。
type Server struct {
	// Addr 是监听地址（如 127.0.0.1:9090）。空表示不启用。
	Addr string
	// AllowPublic 允许监听非回环地址。默认 false，避免无意暴露统计信息。
	AllowPublic bool
	// Providers 是要聚合的统计源（可为多实例）。
	Providers []Provider
	// Logger 用于输出日志，nil 时使用标准库默认 logger。
	Logger *log.Logger
}

func (s *Server) logf(format string, args ...any) {
	if s.Logger != nil {
		s.Logger.Printf(format, args...)
		return
	}
	log.Printf(format, args...)
}

// Handler 返回该统计端点的 http.Handler，便于测试直接挂载。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/stats", s.handleStats)
	mux.HandleFunc("/metrics", s.handleMetrics)
	return mux
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	snap := Aggregate(s.Providers)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(struct {
		Snapshot
		UptimeSeconds float64 `json:"uptime_seconds"`
	}{Snapshot: snap, UptimeSeconds: snap.Uptime.Seconds()})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	snap := Aggregate(s.Providers)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	writeMetric(w, "portmap_active_connections", "Currently active connections.", "gauge", snap.ActiveConns)
	writeMetric(w, "portmap_total_connections", "Total connections handled.", "counter", snap.TotalConns)
	writeMetric(w, "portmap_rejected_connections", "Connections rejected due to the connection limit.", "counter", snap.RejectedConns)
	writeMetric(w, "portmap_dial_errors", "Outbound dial errors.", "counter", snap.DialErrors)
	writeMetric(w, "portmap_up_bytes", "Bytes transferred from clients to targets (upstream).", "counter", snap.UpBytes)
	writeMetric(w, "portmap_down_bytes", "Bytes transferred from targets to clients (downstream).", "counter", snap.DownBytes)
	writeMetric(w, "portmap_uptime_seconds", "Process uptime in seconds.", "gauge", int64(snap.Uptime.Seconds()))
}

func writeMetric(w http.ResponseWriter, name, help, typ string, value int64) {
	_, _ = fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %d\n", name, help, name, typ, name, value)
}

// ListenAndServe 启动统计 HTTP 端点并阻塞，直到 ctx 取消（优雅关闭）或发生
// 致命错误。若 Addr 为空则视为未启用，直接返回 nil。默认仅允许回环地址，
// 非回环需 AllowPublic=true。
func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.Addr == "" {
		return nil
	}
	if !s.AllowPublic && !isLoopbackHost(s.Addr) {
		return errors.New(i18n.T(i18n.KeyErrStatsHTTPPublic, s.Addr))
	}

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrStatsHTTPServe), err)
	}

	srv := &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	s.logf(i18n.T(i18n.KeyLogStatsHTTPStarted), ln.Addr())
	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		s.logf("%s", i18n.T(i18n.KeyLogStatsHTTPStopped))
		return nil
	}
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrStatsHTTPServe), err)
	}
	return nil
}
