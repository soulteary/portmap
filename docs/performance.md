# 性能压测

仓库自带一个独立、自包含的压测工具 [`cmd/loadtest`](../cmd/loadtest/main.go)，用于评估 portmap 转发的**可靠性**与**吞吐量**，并输出当前**主机环境**信息。

默认自包含模式下，工具会在进程内启动一个 echo 目标服务，再用 `forward.New(...)` 起一个转发服务，然后由压测客户端对转发端口发压，形成 `client -> portmap -> echo` 的完整链路，无需任何额外准备即可运行。也可用 `-external <addr>` 直接压测外部已运行的 portmap（此时需自备目标服务）。

外部模式要求目标实现所选协议的回显：原样返回收到的字节。普通 HTTP 服务不符合该负载协议。以下 `go run` 命令在仓库根目录执行。

```bash
# TCP 吞吐模式（长连接持续收发）
go run ./cmd/loadtest -proto tcp -mode throughput -conns 100 -duration 10s

# TCP 连接速率模式（短连接建立/关闭循环）
go run ./cmd/loadtest -proto tcp -mode connrate -conns 100 -duration 10s

# UDP 吞吐模式
go run ./cmd/loadtest -proto udp -mode throughput -conns 50 -duration 10s -payload 512

# 每连接最多完成 1000 个成功请求，并以 30s 为硬截止时间；同时测试限流/超时路径
go run ./cmd/loadtest -proto tcp -conns 20 -requests 1000 -duration 30s -max-conns 200 -idle-timeout 5m

# 压测外部已运行的 portmap（需自备目标服务）
go run ./cmd/loadtest -external 127.0.0.1:13000 -proto tcp -duration 10s

# CI：输出 JSON，并在错误率超过 0.1% 或 p95 超过 5ms 时退出 1
go run ./cmd/loadtest -format json -max-error-rate 0.1 -max-p95 5ms
```

参数说明：

```text
loadtest [flags]

flags:
  -proto tcp|udp            转发协议 (默认 tcp)
  -conns int                并发连接/会话数 (默认 50)
  -duration duration        压测最长时长，所有模式的硬截止时间 (默认 10s)
  -requests int             每连接成功请求数，0 表示仅按 duration 持续跑 (默认 0)
  -payload int              单次请求负载字节数 (默认 1024)
  -mode throughput|connrate 吞吐模式（长连接持续收发）或连接速率模式（短连接循环） (默认 throughput)
  -external string          外部 portmap 地址；为空则自建链路
  -max-conns int            内建转发服务最大并发连接数，0 表示不限制
  -idle-timeout duration    内建转发服务空闲超时；TCP 的 0 表示禁用，UDP 的 0 表示 60s
  -warmup duration          预热时间，预热期数据不计入统计 (默认 1s)
  -format text|json         输出格式 (默认 text)
  -max-samples int          最多保留的 RTT 样本数，使用蓄水池采样 (默认 100000)
  -max-error-rate float     允许的最大错误率百分比，设置后超限退出 1
  -max-p95 duration         允许的最大 p95 RTT，0 表示不设置阈值
```

`-requests` 是成功请求数的提前结束条件，并不会禁用 `-duration`。即使目标持续
拨号失败、超时或返回错误，压测也会在 `-duration` 到期时结束；连续失败会短暂
退避，避免不可达目标导致忙循环。

全局截止时间到达时，已经开始的 I/O 若失败仍会计入错误。分析短时测试末尾的超时时，
需结合发生时间判断；可设置 `-requests`，让成功请求数在截止时间前达到上限，辅助区分结束边界与持续故障。

报告包含三块：**主机环境**（GOOS/GOARCH、CPU 数、Go 版本、主机名、内存分配）、**配置**、**结果**（吞吐 MB/s 与 Gbps、req/s、connrate 模式下的 conns/s、延迟 min/p50/p95/p99/max、错误率与分类计数，以及压测后 `ActiveConns()` 是否归零的可靠性校验）。RTT 使用有界蓄水池采样，长时间压测不会随请求数持续增长内存；`-format json` 提供机器可读结果。

输出格式示例（数值仅作示意）：

```text
==================== portmap loadtest report ====================
[ Host / Runtime ]
  hostname     : example
  os/arch      : linux/amd64
  num cpu      : 8
  go version   : go1.27.x
  ...
[ Results ]
  throughput   : 75.35 MB/s | 0.632 Gbps
  req/s        : 77159.59
[ Latency (RTT) ]
  min          : 26µs
  p50          : 240µs
  p95          : 387µs
  p99          : 553µs
  max          : 19.973ms
[ Reliability ]
  errors       : 0 (rate 0.0000%)
  active conns : 0 (returned to zero) OK
=================================================================
```

## 对比统计上报的开销

默认自包含负载只启动进程内 forward，不创建上报任务，因此不能单独衡量集中上报的开销。
使用 `-external`，对相同版本的独立转发进程比较关闭上报、正常上报、收集端失联三种状态。

先在独立终端启动 TCP echo 目标（需要 Python 3）：

```bash
python3 - <<'PYTHON'
import socketserver

class Echo(socketserver.BaseRequestHandler):
    def handle(self):
        while True:
            data = self.request.recv(65536)
            if not data:
                return
            self.request.sendall(data)

class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True

with Server(("127.0.0.1", 18080), Echo) as server:
    server.serve_forever()
PYTHON
```

第一组不启用上报，在另一个终端启动：

```bash
./portmap forward -listen-host 127.0.0.1 -listen-port 13000 \
  -target 127.0.0.1:18080
```

第二组先停止第一组转发进程，保留 echo 服务，启动收集端：

```bash
mkdir -p .portmap-stats
chmod 700 .portmap-stats
endpoint="unix://$(pwd)/.portmap-stats/stats.sock"
./portmap stats -listen "$endpoint" \
  -journal ./.portmap-stats/performance.journal -http-addr 127.0.0.1:9080
```

再开一个终端，将 `endpoint` 设置为上面相同的绝对路径 URL，启动启用上报的转发进程：

```bash
./portmap forward -listen-host 127.0.0.1 -listen-port 13000 \
  -target 127.0.0.1:18080 -stats-endpoint "$endpoint"
```

在负载终端，对两组使用相同命令，分别保存输出：

```bash
go run ./cmd/loadtest -external 127.0.0.1:13000 -proto tcp \
  -conns 100 -payload 32768 -warmup 2s -duration 30s -format json
```

第三组保持启用上报的转发进程运行，在压测过程中停止收集端，观察同一批长连接是否继续传输。
也可在收集端未启动时重新开始一轮压测。若需要比较 TCP 传输，统一将收集端和转发进程的 URL 改为
`tcp://127.0.0.1:9190`，其他参数保持一致。

比较吞吐、错误率和 RTT 分位数；同时用系统工具记录转发进程和收集端各自的 CPU、RSS 及日志磁盘增长。
压测报告中的运行时内存属于负载工具进程，不能代替独立转发进程的资源数据。
示例 echo 服务和负载工具也消耗 CPU；测量转发器上限时，应排除它们成为瓶颈的情况。
各状态至少重复三轮，并保持主机、负载、空闲超时和日志设置一致。默认日志按连接输出，短连接压测需单独记录其影响。

上述对比衡量同一版本的上报开销。比较实时计数改造前后的吞吐，需要分别构建对应提交，
尤其要区分 Linux 原生 TCP 快速路径与用户态复制，不能仅通过关闭 `-stats-endpoint` 恢复旧复制路径。
这些步骤不是性能结果，不预设可接受的损耗比例。存储与故障边界见 [集中统计指南](stats.md)。

[返回使用说明](../README.md)
