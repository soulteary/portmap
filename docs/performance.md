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

[返回使用说明](../README.md)
