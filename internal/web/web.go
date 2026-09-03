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

// Package web 提供一个可选的、默认关闭、默认仅回环的 Web 面板 HTTP 服务。
// 它复用 stats 包的统计基础设施（通过 stats.Aggregate 聚合多实例快照），并
// 展示 stats.EventLog 中记录的最近连接事件。页面为纯内嵌 HTML + 原生 fetch
// 轮询，无第三方前端依赖，所有文案经 i18n 在服务端注入以跟随 -lang。
package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/soulteary/portmap/internal/i18n"
	"github.com/soulteary/portmap/internal/stats"
)

// isLoopbackHost 判断监听地址的 host 是否为回环地址。空 host（如 ":9090"）视为
// 非回环（等价于所有网卡）。此逻辑复制自 internal/stats/http.go，以避免修改
// stats 包的导出面。
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

// Server 是一个可选的、默认关闭、默认仅回环的 Web 面板 HTTP 服务。
// 暴露 /（内嵌 HTML 面板）、/api/stats（聚合统计 JSON）与 /api/logs（连接事件
// JSON）三个只读端点。
type Server struct {
	// Addr 是监听地址（如 127.0.0.1:8080）。空表示不启用。
	Addr string
	// AllowPublic 允许监听非回环地址。默认 false，避免无意暴露访问日志。
	AllowPublic bool
	// Providers 是要聚合的统计源（可为多实例）。
	Providers []stats.Provider
	// Events 是连接事件日志（环形缓冲）。可为 nil，此时 /api/logs 返回空数组。
	Events *stats.EventLog
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

// Handler 返回该 Web 面板的 http.Handler，便于测试直接挂载。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/logs", s.handleLogs)
	return mux
}

// handleIndex 渲染内嵌的 HTML 面板，所有文案经 i18n 注入。仅精确匹配根路径，
// 其余未知路径返回 404。
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, renderIndexHTML())
}

// handleStats 返回聚合后的统计快照 JSON，与 stats.http handleStats 一致，额外
// 展开 uptime_seconds（浮点秒）字段。
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	snap := stats.Aggregate(s.Providers)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(struct {
		stats.Snapshot
		UptimeSeconds float64 `json:"uptime_seconds"`
	}{Snapshot: snap, UptimeSeconds: snap.Uptime.Seconds()})
}

// handleLogs 返回连接事件的 JSON 数组。Events 为 nil 时返回空数组 []。
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	var events []stats.Event
	if s.Events != nil {
		events = s.Events.Snapshot()
	}
	if events == nil {
		events = []stats.Event{}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	_ = enc.Encode(events)
}

// ListenAndServe 启动 Web 面板 HTTP 服务并阻塞，直到 ctx 取消（优雅关闭）或发生
// 致命错误。若 Addr 为空则视为未启用，直接返回 nil。默认仅允许回环地址，
// 非回环需 AllowPublic=true。
func (s *Server) ListenAndServe(ctx context.Context) error {
	if s.Addr == "" {
		return nil
	}
	if !s.AllowPublic && !isLoopbackHost(s.Addr) {
		return errors.New(i18n.T(i18n.KeyErrWebPublic, s.Addr))
	}

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrWebServe), err)
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

	s.logf(i18n.T(i18n.KeyLogWebStarted), ln.Addr())
	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		s.logf("%s", i18n.T(i18n.KeyLogWebStopped))
		return nil
	}
	if err != nil {
		return fmt.Errorf(i18n.T(i18n.KeyErrWebServe), err)
	}
	return nil
}

// renderIndexHTML 使用 i18n 文案填充内嵌 HTML 模板，返回完整页面字符串。
// 所有注入的文案先经 html.EscapeString 转义，避免破坏页面结构。
func renderIndexHTML() string {
	esc := func(key string) string { return html.EscapeString(i18n.T(key)) }
	return fmt.Sprintf(indexHTMLTemplate,
		esc(i18n.KeyWebTitle),         // 1  <title> 与 <h1>
		esc(i18n.KeyWebAutoRefresh),   // 2  自动刷新 label
		esc(i18n.KeyWebBtnRefresh),    // 3  刷新按钮
		esc(i18n.KeyWebPerfSection),   // 4  性能区标题
		esc(i18n.KeyWebActiveConns),   // 5  活跃连接
		esc(i18n.KeyWebTotalConns),    // 6  累计连接
		esc(i18n.KeyWebRejectedConns), // 7  拒绝连接
		esc(i18n.KeyWebDialErrors),    // 8  拨号失败
		esc(i18n.KeyWebUpBytes),       // 9  上行字节
		esc(i18n.KeyWebDownBytes),     // 10 下行字节
		esc(i18n.KeyWebUptime),        // 11 运行时长
		esc(i18n.KeyWebLogsSection),   // 12 日志区标题
		esc(i18n.KeyWebColTime),       // 13 列：时间
		esc(i18n.KeyWebColKind),       // 14 列：类型
		esc(i18n.KeyWebColProto),      // 15 列：协议
		esc(i18n.KeyWebColClient),     // 16 列：客户端
		esc(i18n.KeyWebColTarget),     // 17 列：目标
		esc(i18n.KeyWebColUp),         // 18 列：上行
		esc(i18n.KeyWebColDown),       // 19 列：下行
		esc(i18n.KeyWebColDuration),   // 20 列：时长
		esc(i18n.KeyWebEmpty),         // 21 空表提示
		esc(i18n.KeyWebCountUnit),     // 22 计数单位（JS）
	)
}

// indexHTMLTemplate 是内嵌面板的 HTML 模板（暗色主题，风格参考 speedup 日志页）。
// 使用 %[n]s 显式索引占位符注入 i18n 文案；JS 中出现的 % 已转义为 %%。
const indexHTMLTemplate = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%[1]s</title>
  <style>
    * { box-sizing: border-box; }
    body { font-family: ui-monospace, "Cascadia Code", "SF Mono", Monaco, monospace; margin: 0; background: #0d1117; color: #c9d1d9; min-height: 100vh; }
    .header { padding: 12px 20px; background: #161b22; border-bottom: 1px solid #21262d; display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
    h1 { margin: 0; font-size: 1.25rem; font-weight: 600; }
    h2 { margin: 0 0 12px; font-size: 1rem; font-weight: 600; color: #8b949e; }
    .toolbar { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
    label { display: flex; align-items: center; gap: 6px; cursor: pointer; font-size: 13px; }
    input[type="checkbox"] { accent-color: #58a6ff; }
    .btn { padding: 6px 12px; font-size: 13px; border: 1px solid #30363d; background: #21262d; color: #c9d1d9; border-radius: 6px; cursor: pointer; font-family: inherit; }
    .btn:hover { background: #30363d; border-color: #8b949e; }
    .count { color: #8b949e; font-size: 13px; }
    .section { margin: 12px; padding: 16px; background: #161b22; border: 1px solid #21262d; border-radius: 8px; }
    .cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 12px; }
    .card { padding: 12px 14px; background: #0d1117; border: 1px solid #21262d; border-radius: 6px; }
    .card .label { color: #8b949e; font-size: 12px; margin-bottom: 6px; }
    .card .value { font-size: 1.4rem; font-weight: 600; color: #58a6ff; word-break: break-all; }
    table { width: 100%%; border-collapse: collapse; font-size: 13px; }
    th, td { padding: 6px 8px; text-align: left; border-bottom: 1px solid #21262d; white-space: nowrap; }
    th { color: #8b949e; font-weight: 600; position: sticky; top: 0; background: #161b22; }
    td { word-break: break-all; }
    tr:nth-child(even) td { background: rgba(255,255,255,0.02); }
    .table-wrap { max-height: calc(100vh - 360px); overflow: auto; }
    .empty { color: #8b949e; padding: 12px 8px; }
    .err { color: #f85149; padding: 12px 8px; }
    .kind-open { color: #7ee787; }
    .kind-close { color: #8b949e; }
    .kind-reject { color: #d29922; }
    .kind-dial-error { color: #f85149; }
  </style>
</head>
<body>
  <div class="header">
    <h1>%[1]s</h1>
    <div class="toolbar">
      <label><input type="checkbox" id="autoRefresh" checked> <span>%[2]s</span></label>
      <button type="button" class="btn" id="btnRefresh">%[3]s</button>
      <span class="count" id="count"></span>
    </div>
  </div>

  <div class="section">
    <h2>%[4]s</h2>
    <div class="cards">
      <div class="card"><div class="label">%[5]s</div><div class="value" id="s-active">-</div></div>
      <div class="card"><div class="label">%[6]s</div><div class="value" id="s-total">-</div></div>
      <div class="card"><div class="label">%[7]s</div><div class="value" id="s-rejected">-</div></div>
      <div class="card"><div class="label">%[8]s</div><div class="value" id="s-dialerr">-</div></div>
      <div class="card"><div class="label">%[9]s</div><div class="value" id="s-up">-</div></div>
      <div class="card"><div class="label">%[10]s</div><div class="value" id="s-down">-</div></div>
      <div class="card"><div class="label">%[11]s</div><div class="value" id="s-uptime">-</div></div>
    </div>
  </div>

  <div class="section">
    <h2>%[12]s</h2>
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th>%[13]s</th><th>%[14]s</th><th>%[15]s</th><th>%[16]s</th>
            <th>%[17]s</th><th>%[18]s</th><th>%[19]s</th><th>%[20]s</th>
          </tr>
        </thead>
        <tbody id="logBody"></tbody>
      </table>
      <div class="empty" id="logEmpty" style="display:none">%[21]s</div>
    </div>
  </div>

  <script>
    var COUNT_UNIT = "%[22]s";
    var EMPTY_TEXT = "%[21]s";
    var body = document.getElementById('logBody');
    var emptyEl = document.getElementById('logEmpty');
    var countEl = document.getElementById('count');
    var autoRefresh = document.getElementById('autoRefresh');
    var timer = null;

    function escapeHtml(s) {
      var div = document.createElement('div');
      div.textContent = (s == null ? '' : String(s));
      return div.innerHTML;
    }

    function fmtBytes(n) {
      n = Number(n) || 0;
      var units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
      var i = 0;
      while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
      return (i === 0 ? n : n.toFixed(2)) + ' ' + units[i];
    }

    function fmtUptime(sec) {
      sec = Math.floor(Number(sec) || 0);
      var d = Math.floor(sec / 86400); sec -= d * 86400;
      var h = Math.floor(sec / 3600); sec -= h * 3600;
      var m = Math.floor(sec / 60); sec -= m * 60;
      var parts = [];
      if (d) parts.push(d + 'd');
      if (h || d) parts.push(h + 'h');
      if (m || h || d) parts.push(m + 'm');
      parts.push(sec + 's');
      return parts.join(' ');
    }

    function loadStats() {
      fetch('/api/stats').then(function(r) {
        if (!r.ok) throw new Error(r.status);
        return r.json();
      }).then(function(s) {
        document.getElementById('s-active').textContent = s.active_conns;
        document.getElementById('s-total').textContent = s.total_conns;
        document.getElementById('s-rejected').textContent = s.rejected_conns;
        document.getElementById('s-dialerr').textContent = s.dial_errors;
        document.getElementById('s-up').textContent = fmtBytes(s.up_bytes);
        document.getElementById('s-down').textContent = fmtBytes(s.down_bytes);
        document.getElementById('s-uptime').textContent = fmtUptime(s.uptime_seconds);
      }).catch(function() {});
    }

    function loadLogs() {
      fetch('/api/logs').then(function(r) {
        if (!r.ok) throw new Error(r.status);
        return r.json();
      }).then(function(entries) {
        countEl.textContent = entries.length + ' ' + COUNT_UNIT;
        if (!entries.length) {
          body.innerHTML = '';
          emptyEl.style.display = '';
          emptyEl.textContent = EMPTY_TEXT;
          return;
        }
        emptyEl.style.display = 'none';
        var rows = [];
        for (var i = entries.length - 1; i >= 0; i--) {
          var e = entries[i];
          var kind = e.kind || '';
          rows.push(
            '<tr>' +
            '<td>' + escapeHtml(e.time) + '</td>' +
            '<td class="kind-' + escapeHtml(kind) + '">' + escapeHtml(kind) + '</td>' +
            '<td>' + escapeHtml(e.proto) + '</td>' +
            '<td>' + escapeHtml(e.client) + '</td>' +
            '<td>' + escapeHtml(e.target) + '</td>' +
            '<td>' + fmtBytes(e.up_bytes) + '</td>' +
            '<td>' + fmtBytes(e.down_bytes) + '</td>' +
            '<td>' + (Number(e.duration_ms) || 0) + ' ms</td>' +
            '</tr>'
          );
        }
        body.innerHTML = rows.join('');
      }).catch(function() {
        countEl.textContent = '—';
      });
    }

    function load() { loadStats(); loadLogs(); }

    function schedule() {
      if (timer) clearInterval(timer);
      if (autoRefresh.checked) timer = setInterval(load, 2000);
    }

    autoRefresh.addEventListener('change', schedule);
    document.getElementById('btnRefresh').addEventListener('click', function() { load(); });
    load();
    schedule();
  </script>
</body>
</html>
`
