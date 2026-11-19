# 集中统计

[中文](stats.md) | [English](stats.en.md)

`portmap`、`portmap forward` 和 `portmap proxy` 在连接未关闭时，按实际成功写出的字节更新本地计数。
启用上报后，每个进程通过一条持久连接，每秒主动发送一批累计快照。收集端不会轮询转发进程。
现有 `-stats-addr`、`-web-addr` 本地查询服务继续可用。

## 同机使用 Unix socket

先创建私有目录，再启动收集端：

```sh
mkdir -p .portmap-stats
chmod 700 .portmap-stats
endpoint="unix://$(pwd)/.portmap-stats/stats.sock"
./portmap stats -listen "$endpoint" \
  -journal ./.portmap-stats/history.journal -http-addr 127.0.0.1:9080
```

在转发进程所在终端，将 `endpoint` 设置为同一个绝对路径 URL，再按需运行以下命令之一：

```sh
./portmap forward -listen-host 127.0.0.1 -listen-port 31008 \
  -target 127.0.0.1:8080 -stats-endpoint "$endpoint"
./portmap proxy -config ./config.yaml -stats-endpoint "$endpoint"
sudo ./portmap -config ./config.yaml -stats-endpoint "$endpoint"
```

三条命令是独立示例，目标服务或配置文件需提前准备。SSH 上游转发使用相同的上报机制。
`socat` 模式不支持上报，`stats_endpoint` 或 `-stats-endpoint` 非空时会返回错误。

未指定 `-listen` 时，收集端在系统临时目录下创建当前用户的私有目录，并在启动日志中打印 socket 地址。
转发进程始终需要显式配置收集端地址。Unix socket 权限为 `0600`，应使用同一账户或 root 连接。
自定义 socket 的父目录需提前创建。启动时不会自动删除已有 socket；崩溃后应先确认没有收集端监听，再删除残留文件。

`stats_endpoint` 写在 `forward` 或 `proxy` 段内，旧版平铺 forward 配置也支持该字段：

```yaml
forward:
  listen_port: 31008
  target: 127.0.0.1:8080
  stats_endpoint: unix:///absolute/private/directory/stats.sock
proxy:
  addr: 127.0.0.1:8118
  stats_endpoint: unix:///absolute/private/directory/stats.sock
```

多实例上报是进程级设置：非空地址必须一致，一个批次包含全部实例。
显式 CLI 参数覆盖实例配置；`-stats-endpoint=''` 可禁用上报。
实例 ID 是本次进程运行中的列表位置，标签包含监听地址，forward 标签还包含目标地址。
进程重启后使用新的 boot ID，避免与上一次运行的计数混淆。

## 使用 TCP

```sh
./portmap stats -listen tcp://127.0.0.1:9190 \
  -journal ./history.journal -http-addr 127.0.0.1:9080
./portmap proxy -config ./config.yaml -stats-endpoint tcp://127.0.0.1:9190
```

收集端默认要求 TCP 监听地址为回环 IP。跨机器使用时，`-allow-public` 显式允许其他监听地址，
应部署在可信网络中或通过另行保护的隧道访问。协议会协商版本，但没有内置身份认证或加密；
端口可达不代表对端身份可信。HTTP 查询监听器始终要求回环 IP，`-http-addr=''` 可关闭查询服务。

## 命令参数

用 `portmap stats -help` 查看已安装版本的选项。统计服务只接受命令行参数，
不读取 `-config` 或 YAML 中的 `stats:` 段。

- `-listen`：上报接收 URL，默认是系统临时目录下的每用户私有 Unix socket。
- `-journal`：追加日志文件，默认是工作目录下的 `portmap-stats.journal`，父目录必须存在。
- `-http-addr`：查询地址，默认 `127.0.0.1:9080`，留空关闭。
- `-allow-public`：允许非回环 TCP 上报接收地址，不会开放 HTTP 查询监听器。
- `-lang`：帮助和日志使用的语言。

`-stats-endpoint` 属于转发或代理进程；`-stats-addr`、`-web-addr` 用于提供这些进程的本地查询服务，
不能用来设置集中收集端。

## 查询和存储

在浏览器打开 `http://127.0.0.1:9080/`。同一 HTTP 监听器提供：

- `/stats`、`/api/stats`：聚合计数。
- `/metrics`：聚合 Prometheus 指标。
- `/api/logs`：最近 1000 条连接事件。
- `/api/sources`：每个进程最后收到的批次，包含接收端时间戳。
- `/api/history`：最近 1000 个已接受批次，不包含事件内容。

超过三秒没有更新的来源，不再计入聚合活跃连接数，但其累计计数继续保留。
来源记录保留最后上报的活跃连接数，读取 `/api/sources` 时应检查 `received_at`，再判断数据是否仍有效。

追加日志使用带长度前缀的 JSON 记录，并加锁防止多个收集端同时使用同一文件。
每个批次持久化并同步到磁盘后才发送确认。重启时重放日志，恢复序号、累计基线、最近历史和事件。
末尾不完整的帧会被丢弃，格式错误的帧或无效快照会阻止启动。
重复或更旧的序号不会追加记录，同一次进程启动内累计计数下降会被拒绝。
收集端对各进程最新累计快照求和，不会将每个批次的累计值重复相加。

当前不自动压缩或轮转日志。需规划磁盘保留策略，并在归档或替换文件前停止收集端。
使用空日志重新启动会重置收集端历史。每份日志最多保留 10,000 次进程运行，
每批最多支持 1024 个实例；接收端最多同时处理 128 条连接，每帧上限为 4 MiB。

## 故障和计量范围

- 转发不等待收集端。收集端缺席、缓慢、版本不兼容或断线时，在后台按指数退避加随机抖动重试。
  重试间隔最多约 38 秒，另计拨号和握手时间。
- 断线后只需发送最新累计快照，就能补齐总量；无法据此重建断线期间精确的逐秒流量分布。
- 连接事件共享有界本地日志，容量由 `web_log_max` 控制，默认 1000 条。每批最多发送 1000 条待上报事件，
  超出部分计入 `events_dropped`。事件为尽力交付；确认丢失时，该丢弃数可能保守地包含已被收集端保存的事件。
- 转发进程不做本地持久化，退出时也不等待最后一次上报。进程退出可能丢失最后一次已接受采样之后的流量；
  正常情况下这个窗口约为一秒，收集端故障时可能更长。
- 上报中的客户端地址、目标地址和实例标签限制为 256 字节，本地日志保留原值。
  收集端事件包含进程 `source` ID，因为连接编号只在单个进程内唯一。
- 字节计数按成功写出的数量累计，包含部分写入。不含 SSH 封装、加密及 TCP/IP 开销。
  代理协商回复不计入隧道负载，普通 HTTP 统计改写后转发的消息。
- 实时计数使用复用的 32 KiB 用户态复制缓冲。计量转发不再使用原生 TCP `io.Copy` / Linux splice 快速路径，
  即使关闭空闲超时也是如此；本改动不宣称提升吞吐量。

[返回使用说明](../README.md)
