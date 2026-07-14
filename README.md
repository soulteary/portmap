# portmap

[![CI](https://github.com/soulteary/portmap/actions/workflows/ci.yml/badge.svg)](https://github.com/soulteary/portmap/actions/workflows/ci.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/soulteary/portmap)](https://goreportcard.com/report/github.com/soulteary/portmap) [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

<p align="center">
  <a href="README.en.md">ENGLISH</a> | <a href="README.md" target="_blank">中文文档</a>
</p>

<p align="center">
  <img src=".github/workflows/assets/portmap-logo.png" alt="portmap Logo" width="160"/>
</p>

> 一个轻量的 **TCP/UDP 端口转发小工具** —— 用 Go 实现，无需依赖系统 `socat`。

<p align="center">
  <img src=".github/workflows/assets/portmap-banner.jpg" alt="portmap Banner" width="720"/>
</p>

## 概览

`portmap` 是一个用 Go 实现的通用 TCP/UDP 端口转发小工具，等价于：

```bash
sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222
```

支持两种模式：

- **`go`（默认）**：纯 Go 实现，不依赖系统 `socat`，跨平台，对应 `TCP-LISTEN`/`fork`/`reuseaddr`，并扩展了 UDP、并发限流、空闲超时与连接级日志（见下）。
- **`socat`**：直接调用本机的 `socat` 命令（可选 `-sudo`），生成等价命令行（支持 TCP/UDP）。

## 为什么写这个工具

Podman（尤其是 rootless 模式）默认不允许非特权用户监听 1024 以下的低位端口，这与 Docker 的端口映射行为存在明显差异：在 Docker 里可以直接把容器映射到宿主机的 22/53/80 等低位端口，而在 Podman rootless 下同样的映射会因为权限限制而失败。

常见的绕过办法是调整系统设置，例如降低 `sysctl net.ipv4.ip_unprivileged_port_start`，但这会修改全局内核参数、影响整机安全边界，并不总是可取。

`portmap` 提供了一条更轻量的路径：让容器以 rootless 方式监听一个高位端口（如 2222），再用本工具在宿主机上把低位端口（如 22/53/80）转发到该高位端口。这样既保留了类 Docker 的端口映射体验，又无需调整 `net.ipv4.ip_unprivileged_port_start` 等系统级设置。

Podman 示例：rootless 容器监听 `2222`，用 `portmap` 把宿主机 22 端口暴露出去（监听 22 需要特权，故用 `sudo`）：

```bash
# rootless 容器（示例）：将服务映射到高位端口 2222
podman run -d -p 2222:22 your-image

# 用 portmap 把低位端口 22 转发到容器监听的 2222
sudo portmap -listen-port 22 -target 127.0.0.1:2222
```

## 构建

```bash
go build -o portmap .
# 或使用 Makefile（自动注入版本信息）
make build
```

## 用法

```text
portmap [flags]

flags:
  -listen-port int        本地监听端口 (默认 22)
  -listen-host string     本地监听地址（默认所有网卡）
  -target string          转发目标地址 host:port (默认 "127.0.0.1:2222")
  -mode string            转发模式：go 或 socat (默认 "go")
  -proto string           转发协议：tcp 或 udp (默认 "tcp")
  -reuseaddr              启用 SO_REUSEADDR (默认 true)
  -sudo                   socat 模式下以 sudo 运行
  -dial-timeout duration  拨号目标超时 (默认 10s)
  -max-conns int          最大并发连接数，0 表示不限制（仅 go 模式）
  -idle-timeout duration  空闲超时，双向无数据则断开，0 表示不启用（仅 go 模式）
  -log-level string       日志级别：info 或 debug（仅 go 模式，默认 "info"）
  -quiet                  安静模式，抑制每连接的常规日志（仅 go 模式）
  -config string          YAML 配置文件路径
  -version                打印版本信息后退出
```

## 示例

与原命令完全等价（需 root 监听 22 端口）：

```bash
sudo ./portmap -listen-port 22 -target 127.0.0.1:2222
```

复用系统 socat（生成等价命令行）：

```bash
./portmap -mode socat -sudo -listen-port 22 -target 127.0.0.1:2222
```

UDP 转发（如 DNS）：

```bash
./portmap -proto udp -listen-port 53 -target 127.0.0.1:5353
```

高位端口本地测试 + 并发限流 + 空闲超时 + 调试日志：

```bash
./portmap -listen-port 13000 -target 127.0.0.1:2222 \
  -max-conns 100 -idle-timeout 5m -log-level debug
```

查看版本：

```bash
./portmap -version
```

按 `Ctrl+C` 优雅退出，会等待在途连接处理完成。

## 配置文件

除命令行 flag 外，还可通过 `-config <path>` 从 YAML 文件读取配置：

```bash
./portmap -config config.yaml
```

- **优先级**：命令行显式设置的 flag > 配置文件 > 内置默认值。
  即：只有在配置文件中出现、且命令行未显式设置的字段，才会覆盖默认值。
- `dial_timeout` / `idle_timeout` 在 YAML 中用字符串（如 `"10s"`、`"5m"`），经 `time.ParseDuration` 解析。
- 配置文件中出现未知字段会直接报错，便于及早发现拼写错误。
- 启动时会打印一行「生效配置」摘要，便于确认合并后的实际参数。

完整示例见 [config.example.yaml](config.example.yaml)：

```yaml
listen_port: 22
listen_host: ""
target: 127.0.0.1:2222
mode: go
proto: tcp
reuseaddr: true
sudo: false
dial_timeout: 10s
max_conns: 0
idle_timeout: 0s
log_level: info
quiet: false
```

命令行覆盖配置文件示例（配置文件里 `listen_port` 为 22，此处显式指定 8022 生效）：

```bash
./portmap -config config.yaml -listen-port 8022
```

## 模式差异（go vs socat）

两种模式支持的能力不同，请按需选择：

| 参数 / 能力 | `go` 模式 | `socat` 模式 |
| --- | --- | --- |
| `-listen-host` | 支持（绑定指定地址） | 支持（生成 `bind=<host>`） |
| `-proto tcp/udp` | 支持 | 支持 |
| `-reuseaddr` | 支持 | 支持（`reuseaddr`） |
| `-max-conns` | 支持 | **不支持**（忽略并提示） |
| `-idle-timeout` | 支持 | **不支持**（忽略并提示） |
| `-log-level` | 支持 | **不支持**（忽略并提示） |
| `-quiet` | 支持 | **不支持**（忽略并提示） |
| `SIGUSR1` 状态打印 | 支持（见下） | 不适用（由 socat 进程接管） |

- **`-listen-host` 在 socat 下的行为**：非空时会在监听地址上追加 `bind=<host>`，
  例如 `-mode socat -listen-host 127.0.0.1 -listen-port 22` 生成
  `socat TCP-LISTEN:22,bind=127.0.0.1,fork,reuseaddr TCP:127.0.0.1:2222`；
  不设置时行为与之前完全一致（仅监听端口，等价 `0.0.0.0`）。
- 在 socat 模式下显式设置了仅 `go` 模式支持的参数（`-idle-timeout`/`-max-conns`/
  `-log-level`/`-quiet`）时，程序会打印一行提示说明这些参数被忽略。

## UDP 行为说明

UDP 无连接，`go` 模式以「客户端地址 → 一条到目标的 UDP 连接」维护会话表，目标的回包按会话转发回对应客户端。以下参数在 UDP 下与 TCP 的差异：

- **`-max-conns`**：UDP 下限制**并发会话数**（而非连接数），超限时直接拒绝新客户端并丢弃其首包（UDP 无排队语义），并在 `-log-level debug` 下记录。
- **`-idle-timeout`**：UDP 下为 `0` 会**回退为默认 60s**，即回收空闲超过 60s 的会话（TCP 下 `0` 表示不启用）。

## 可观测性

`go` 模式下：

- 每条连接在建立与关闭时各打印一行日志，包含连接序号、双方地址、上/下行字节数与连接时长。
- 维护活跃连接的原子计数。
- 在类 Unix 平台可向进程发送 `SIGUSR1` 打印当前连接快照，例如：

```bash
kill -USR1 <pid>
# 日志输出：status: active=<当前活跃连接数> total=<累计处理连接数>
```

  Windows 无 `SIGUSR1`，该功能自动跳过（不影响编译与运行）。

- `-quiet` 抑制常规日志；`-log-level debug` 输出更详细信息（如 `pipe` 层异常）。

## 工程化

```bash
make build     # 注入版本信息编译
make test      # go test ./... -race
make vet       # go vet
make lint      # golangci-lint（需已安装）
make release   # 交叉编译多平台产物到 dist/
```

CI 见 `.github/workflows/ci.yml`：在 linux/macOS/windows 上运行 `go vet` 与
`go test -race`，并独立运行 `golangci-lint`。

## 项目结构

```text
.
├── main.go                          # 命令行入口与参数解析
├── main_test.go                     # 命令行入口测试
├── config.go                        # YAML 配置文件加载与合并
├── config_test.go                   # 配置文件加载/合并测试
├── config.example.yaml              # 配置文件示例
├── signals_unix.go                  # 类 Unix 平台 SIGUSR1 状态打印
├── signals_windows.go               # Windows 平台 no-op（无 SIGUSR1）
├── Makefile                         # 构建/测试/发布
├── LICENSE                          # Apache 2.0 许可证
├── .golangci.yml                    # golangci-lint 配置
├── .github/workflows/ci.yml         # CI 工作流
└── internal
    ├── forward                      # 纯 Go TCP/UDP 转发器
    │   ├── forward.go               # TCP 转发、限流、空闲超时、日志
    │   ├── forward_test.go          # forward 单元测试
    │   ├── udp.go                   # UDP 会话转发
    │   ├── reuseaddr_unix.go        # 类 Unix 平台 SO_REUSEADDR
    │   └── reuseaddr_windows.go     # Windows 平台 SO_REUSEADDR
    └── socat                        # 调用系统 socat 的 fallback
        ├── socat.go                 # 构造并执行 socat 命令
        ├── socat_test.go            # socat 单元测试
        ├── socat_cancel_unix.go     # 类 Unix 平台 SIGTERM 优雅取消
        └── socat_cancel_windows.go  # Windows 平台 no-op（无 SIGTERM）
```

## 测试

```bash
go test ./...
# 或
make test
```

## 许可证

本项目基于 [Apache License 2.0](LICENSE) 开源，版权所有 (c) 2026 soulteary。
