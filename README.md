# portmap

[![CI](https://github.com/soulteary/portmap/actions/workflows/ci.yml/badge.svg)](https://github.com/soulteary/portmap/actions/workflows/ci.yml) [![Go Report Card](./.github/goreportcard.svg)](.github/goreportcard-report.md) [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

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

它采用「子命令」结构，除默认的端口转发（`forward`）外，还内置了一个单端口双协议的
**SOCKS5 + HTTP 本地代理**（`proxy` 子命令，应用层代理能力）。整个工具仍保持
**单仓库、单静态二进制、零第三方运行时依赖**的定位。

端口转发支持两种模式：

- **`go`（默认）**：纯 Go 实现，不依赖系统 `socat`，支持 Linux 与 macOS，对应 `TCP-LISTEN`/`fork`/`reuseaddr`，并扩展了 UDP、并发限流、空闲超时与连接级日志（见下）。
- **`socat`**：直接调用本机的 `socat` 命令（可选 `-sudo`），生成等价命令行（支持 TCP/UDP）。

## 1.2.0 版本亮点

- 新增单端口 SOCKS5 + HTTP/HTTPS 代理，并支持 SOCKS5、HTTP CONNECT 与 SSH 上游。
- 新增 forward/proxy YAML 多实例配置。
- 新增 JSON/Prometheus 统计端点、只读 Web 监控面板和结构化连接事件。
- 新增自包含的 TCP/UDP 压测工具，支持有界延迟采样、JSON 输出和 CI 阈值。
- 强化自引用目标、超时、HTTP 请求头、SSH 生命周期和发布流水线安全边界。

1.2.0 支持 Linux/macOS 的 amd64 与 arm64；Windows 支持及 Windows 发布归档
已经移除。完整变更和升级影响见 [CHANGELOG.md](CHANGELOG.md)。

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

需要 Go 1.27+（准确版本以 `go.mod` 为准）。

```bash
go build -o portmap .
# 或使用 Makefile（自动注入版本信息）
make build
```

## Homebrew 安装

macOS / Linux 可通过 [Homebrew Tap](https://github.com/soulteary/homebrew-tap) 安装：

```bash
brew tap soulteary/tap
brew install soulteary/tap/portmap
```

验证：

```bash
portmap --version
# portmap 1.2.0 (commit <short-sha>, built <timestamp>)
```

## 容器镜像

预构建的多架构镜像（`linux/amd64` + `linux/arm64`）发布在 ghcr.io 与 Docker Hub：

```bash
# 从 ghcr.io 拉取
docker pull ghcr.io/soulteary/portmap:latest
# 或从 Docker Hub 拉取
docker pull soulteary/portmap:latest
# 生产环境固定使用 1.2.0
docker pull ghcr.io/soulteary/portmap:1.2.0
```

容器基于 `scratch`，仅包含单个静态二进制。运行时需使用 host 网络才能完成端口转发：

```bash
# 把宿主机 22 转发到容器监听的 2222（低位端口需特权）
docker run --rm --network host ghcr.io/soulteary/portmap:latest \
  -listen-port 22 -target 127.0.0.1:2222
```

## 用法

`portmap` 采用「子命令」结构，共享同一套 `-config` / `-lang` / `-version` 通用参数：

```text
portmap <subcommand> [flags]

subcommands:
  forward   TCP/UDP 端口转发（默认子命令）
  proxy     单端口 SOCKS5 + HTTP 代理
  version   打印版本信息
```

- **向后兼容**：无子命令时（例如第一个参数以 `-` 开头，或无任何参数）默认走 `forward`，
  因此 `portmap -listen-port 22 -target 127.0.0.1:2222` 等历史用法完全不变。
- `-lang` 会在分发前统一处理一次，供所有子命令与 `--help` 共享。

### forward 子命令（端口转发）

```text
portmap [forward] [flags]

flags:
  -listen-port int        本地监听端口 (默认 22)
  -listen-host string     本地监听地址（默认所有网卡）
  -target string          转发目标地址 host:port (默认 "127.0.0.1:2222")
  -mode string            转发模式：go 或 socat (默认 "go")
  -proto string           转发协议：tcp 或 udp (默认 "tcp")
  -reuseaddr              启用 SO_REUSEADDR (默认 true)
  -sudo                   socat 模式下以 sudo 运行
  -dial-timeout duration  拨号目标超时 (默认 10s)
  -max-conns int          最大并发连接数，0 表示不限制（仅 go 模式，默认 256）
  -idle-timeout duration  空闲超时，双向均无数据超过阈值才断开，0 表示不启用（仅 go 模式，默认 5m）
  -log-level string       日志级别：info 或 debug（仅 go 模式，默认 "info"）
  -quiet                  安静模式，抑制每连接的常规日志（仅 go 模式）
  -stats-addr string      可选的 HTTP 统计端点地址（如 127.0.0.1:9090），留空表示关闭；仅允许回环地址
  -web-addr string        可选的 Web 面板监听地址（如 127.0.0.1:8080），留空表示关闭；仅允许回环地址
  -web-log-max int        Web 面板连接事件环形缓冲保留的最大条数 (默认 1000)
  -config string          YAML 配置文件路径（读取 forward: 段）
  -lang string            界面语言：en/zh/ja/ko/fr/de（默认自动检测系统语言）
  -version                打印版本信息后退出
```

为避免异常客户端无界占用文件描述符和 goroutine，forward 默认最多接受 256 个并发 TCP 连接或 UDP 会话，并回收双向空闲超过 5 分钟的会话。需要保留旧版无界行为时，可显式设置 `-max-conns 0 -idle-timeout 0`。

### proxy 子命令（SOCKS5 + HTTP 代理）

在**同一个监听端口**上通过窥探连接首字节自动区分 SOCKS5 与 HTTP/HTTPS 客户端；
出站连接默认**直连**目标，也可显式配置 SOCKS5、HTTP CONNECT 或 SSH 上游；
`HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY` 等环境变量始终被忽略。

```text
portmap proxy [flags]

flags:
  -addr string            监听地址，SOCKS5 与 HTTP 共用此端口 (默认 "127.0.0.1:1080")
  -allow-public           允许监听非回环地址（代理不提供身份认证）
  -dial-timeout duration  出站连接超时时间 (默认 30s)
  -handshake-timeout duration
                          协议握手超时，0 表示不限制 (默认 10s)
  -idle-timeout duration  双向空闲超时，0 表示不限制 (默认 5m0s)
  -max-conns int          代理最大并发连接数，0 表示不限制 (默认 256)
  -upstream string        上游代理 URL，支持 socks5://、http://、ssh://；留空表示直连
  -upstream-agent         启用 ssh-agent 认证（仅 ssh 上游，默认 true）
  -upstream-agent-socket string
                          ssh-agent 的 socket 路径（留空表示读环境变量 SSH_AUTH_SOCK）
  -upstream-identity string
                          SSH 上游认证使用的私钥文件
  -upstream-insecure      跳过 SSH host key 校验（不安全，仅用于自建测试环境）
  -upstream-keepalive duration
                          SSH 主动保活间隔；0 使用默认 30s，负数禁用主动保活
  -upstream-keepalive-max-failures int
                          判定 SSH 上游断线前允许的连续保活失败次数 (默认 3)
  -upstream-known-hosts string
                          SSH host key 校验使用的 known_hosts 文件
  -stats-addr string      可选的 HTTP 统计端点地址（如 127.0.0.1:9090），留空表示关闭；除非 -stats-allow-public 否则仅允许回环地址
  -stats-allow-public     允许统计端点监听非回环地址
  -web-addr string        可选的 Web 面板监听地址（如 127.0.0.1:8080），留空表示关闭；除非 -web-allow-public 否则仅允许回环地址
  -web-allow-public       允许 Web 面板监听非回环地址
  -web-log-max int        Web 面板连接事件环形缓冲保留的最大条数 (默认 1000)
  -config string          YAML 配置文件路径（读取 proxy: 段）
  -lang string            界面语言：en/zh/ja/ko/fr/de（默认自动检测系统语言）
  -version                打印版本信息后退出
```

启动一个本地代理（默认监听 `127.0.0.1:1080`）：

```bash
./portmap proxy -addr 127.0.0.1:1080
```

将浏览器/工具的 SOCKS5 或 HTTP 代理指向该地址即可；两种协议共用同一端口。
按 `Ctrl+C` 优雅退出；服务停止接收新连接，并最多等待 10 秒让已有连接结束。
`-handshake-timeout` 仅限制客户端协议探测与请求解析；目标连接使用独立的
`-dial-timeout`。HTTP 请求行与请求头合计最多 1 MiB，超限返回 `431`。

> 安全提示：代理不提供身份认证，默认拒绝监听 `0.0.0.0`、`::` 等非回环地址。
> 只有在网络访问已由防火墙或其它边界保护时，才应显式添加 `-allow-public`。
> 指向代理自身监听地址的 HTTP/SOCKS5 请求会被拒绝，以避免递归连接风暴。
> `-allow-public` 只开放代理监听器；统计端点和 Web 面板必须分别通过
> `-stats-allow-public` 与 `-web-allow-public` 显式开放。

#### SSH 上游 ssh-agent 认证（推荐）

`upstream` 为 `ssh://` 时，portmap 默认尝试通过 ssh-agent 完成认证：签名由 agent 代劳，
加密私钥的口令交给 `ssh-add` / macOS 钥匙串保管，portmap 自身完全不接触口令。

认证方式的尝试顺序为：**ssh-agent → `-upstream-identity` 指定的私钥 → 密码**。

- `-upstream-agent`（默认 `true`）控制是否启用 agent 认证，用 `-upstream-agent=false` 关闭；
- `-upstream-agent-socket` 指定 agent 的 unix socket 路径，留空表示读取环境变量 `SSH_AUTH_SOCK`；
- 配置文件中对应 `proxy:` 段下的 `upstream_agent` 与 `upstream_agent_socket`。

推荐用法是先把私钥加入 agent，之后启动 portmap 不必再输入任何口令：

```bash
# macOS：把加密私钥的口令存入钥匙串，之后（含重启后）都不用再输入
ssh-add --apple-use-keychain ~/.ssh/keys/litchi/litchi-2018

# Linux：加入正在运行的 ssh-agent
ssh-add ~/.ssh/id_rsa

# 之后直接启动即可，不会再有 passphrase / 密码提示
./portmap proxy -upstream ssh://root@host:22
```

> [!NOTE]
> agent 认证虽然默认开启，但对现有配置**完全向后兼容**：`SSH_AUTH_SOCK` 为空或 socket
> 不可达时会静默跳过 agent，不产生任何错误，私钥与密码认证照常工作。
>
> agent 可用时只会**压制交互式终端提示**：环境变量 `PORTMAP_UPSTREAM_PASSWORD`、
> `PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE` 以及配置项 `upstream_identity_passphrase`
> 仍然照常生效，显式提供的凭据不会因为 agent 可用而被忽略。
>
> 若同时配置了 `-upstream-identity`、该私钥是加密私钥且未提供 passphrase，只要 agent 可用，
> portmap 不再像以前那样直接报错退出，而是记录一条告警日志、跳过该私钥并改用 agent 认证。
>
> 判断「是否还需要向用户索要口令」时会真实连接一次 socket 做连通性探测（带 500ms 超时，
> 探测后立即关闭）。因此 `SSH_AUTH_SOCK` 指向一个已失效的 agent（机器重启后 agent 尚未
> 启动、socket 路径残留等）时，agent 会被判为不可用，交互式提示照常出现，无需再用环境变量
> 或 `-upstream-agent=false` 绕开。

> [!WARNING]
> Windows 上的 ssh-agent 通过 named pipe 而非 unix socket 通信，因此不受支持；
> 该平台请继续使用 passphrase 或登录密码的方式（unix socket 探测必然失败，agent 判为
> 不可用，交互式提示照常出现）。

#### SSH 上游私钥 passphrase（证书密码）

当 `upstream` 为 `ssh://` 且 `-upstream-identity` 指向的私钥是**加密私钥**时，
需要提供 passphrase 才能解密。passphrase 按以下优先级解析（高 → 低）：

1. 环境变量 `PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE`；
2. 配置文件字段 `upstream_identity_passphrase`；
3. 交互式终端输入（仅当上面两者都为空、且配置了 `-upstream-identity`、
   且 stdin 是 TTY 时触发，读取时不回显）。

出于安全考虑**不提供命令行 flag**（避免明文出现在进程列表 / shell 历史中）。
**推荐使用环境变量**；若写入 YAML 明文，请自行控制配置文件权限。passphrase
不会进入日志。加密私钥但未提供 passphrase 时会给出明确的错误提示。

ssh-agent 可用时（见上一节）不会触发第 3 步的交互式提示：此时加密私钥缺少 passphrase
只会记录一条告警并改用 agent 认证，而不再报错退出。

```bash
# 环境变量方式（推荐）
export PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE='your-passphrase'
./portmap proxy -upstream ssh://root@host:22 -upstream-identity ~/.ssh/id_rsa
```

#### SSH 上游登录密码

不使用私钥、改用密码登录 SSH 上游时，密码按以下优先级解析（高 → 低）：

1. 上游 URL 中的 userinfo，如 `ssh://root:pass@host:22`（特殊字符需
   percent-encode，例如 `@` 写作 `%40`）；
2. 环境变量 `PORTMAP_UPSTREAM_PASSWORD`；
3. 交互式终端输入（仅当上面两者都为空、且**未**配置 `-upstream-identity`、
   且 stdin 是 TTY 时触发，读取时不回显）。

与 passphrase 一样**不提供命令行 flag**，密码也不会进入日志。私钥与密码两种
认证方式互斥：配置了 `-upstream-identity` 就不会再提示输入密码。stdin 不是 TTY
时（后台运行、systemd、管道等）会跳过提示，直接报缺少认证方式的错误。ssh-agent
可用时（见上文「SSH 上游 ssh-agent 认证」）同样不会提示输入密码。

```bash
# 交互式输入（终端前台运行）
./portmap proxy -upstream ssh://root@host:22

# 环境变量方式（适用于无终端环境）
export PORTMAP_UPSTREAM_PASSWORD='your-password'
./portmap proxy -upstream ssh://root@host:22
```

> [!NOTE]
> 配置文件中的 `upstream_identity` 与 `upstream_known_hosts` 支持 `~` 开头的
> 路径，会展开为当前用户的 home 目录（YAML 不经过 shell，无法依赖 shell 展开）。

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
# 或
./portmap version
```

启动本地 SOCKS5 + HTTP 代理（单端口双协议，出站直连）：

```bash
./portmap proxy -addr 127.0.0.1:1080
```

按 `Ctrl+C` 优雅退出，会等待在途连接处理完成。

## 界面语言

帮助文本、日志与错误消息支持多语言：`en`（英文）、`zh`（简体中文）、`ja`（日文）、`ko`（韩文）、`fr`（法文）、`de`（德文），无法识别时回退英文。

默认从环境变量自动检测，检测优先级为：`PORTMAP_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` > `LANGUAGE`。也可显式覆盖：

```bash
./portmap -lang zh -version          # 命令行显式指定
PORTMAP_LANG=ja ./portmap -version   # 环境变量指定
```

## 配置文件

除命令行 flag 外，还可通过 `-config <path>` 从 YAML 文件读取配置：

```bash
./portmap -config config.yaml
```

- **优先级**：命令行显式设置的 flag > 配置文件 > 内置默认值。
  即：只有在配置文件中出现、且命令行未显式设置的字段，才会覆盖默认值。
- `dial_timeout` / `idle_timeout` 在 YAML 中用字符串（如 `"10s"`、`"5m"`），经 `time.ParseDuration` 解析。
- 配置文件中出现未知字段会直接报错，便于及早发现拼写错误。
- `lang` 字段仅影响运行期消息（日志/错误），**不影响** `--help` 与 flag 描述的语言：`--help` 在配置文件加载前就已定稿，如需改变其语言请用命令行 `-lang`。
- 启动时会打印一行「生效配置」摘要，便于确认合并后的实际参数。

完整示例见 [config.example.yaml](config.example.yaml)：

```yaml
forward:
  listen_port: 22
  listen_host: ""
  target: 127.0.0.1:2222
  mode: go
  proto: tcp
  reuseaddr: true
  sudo: false
  dial_timeout: 10s
  max_conns: 256
  idle_timeout: 5m
  log_level: info
  quiet: false
  stats_addr: ""
  web_addr: ""
  web_log_max: 1000
proxy:
  addr: 127.0.0.1:1080
  dial_timeout: 30s
  max_conns: 256
  handshake_timeout: 10s
  idle_timeout: 5m
  allow_public: false
  stats_addr: ""
  stats_allow_public: false
  web_addr: ""
  web_allow_public: false
  web_log_max: 1000
lang: en
```

- 配置按子命令**分段**：`forward` 子命令读取 `forward:` 段，`proxy` 子命令读取 `proxy:` 段，顶层 `lang` 为两者共享。
- **向后兼容**：旧版「平铺 forward」布局（顶层直接写 `listen_port` / `target` / … ）仍受支持，`portmap` 会自动识别并归一化到 `forward:` 段，无需迁移即可继续使用。

### 多端口映射（多实例）

`forward:` 与 `proxy:` 既可写成**单个对象**（1 个实例），也可写成**对象列表**，用于在一次运行中同时启动多个端口映射（多实例）。列表元素的字段与单对象写法完全一致：

```yaml
forward:
  - listen_port: 22
    target: 127.0.0.1:2222
  - listen_port: 80
    target: 127.0.0.1:8080

proxy:
  - addr: 0.0.0.0:8118
    allow_public: true
    upstream: ssh://root@10.10.10.10
    upstream_identity: ~/.ssh/id_rsa
    upstream_keepalive: 30s
    upstream_keepalive_max_failures: 3
  - addr: 127.0.0.1:1080
    upstream: socks5://user:pass@host:1080
```

- 各实例在各自的 goroutine 中并发启动；收到退出信号（`Ctrl-C` / `SIGTERM`）时统一优雅关闭，任一实例发生致命错误会触发整体退出。
- 多实例**只能在配置文件中表达**；CLI 仍是单实例语义（如 `portmap proxy -addr ... -upstream ...`）。多实例场景下命令行的 per-instance flag（`-listen-port`/`-addr`/`-upstream` 等）无法一一对应，会被忽略并在日志中提示；`-config`/`-lang` 仍生效。forward 的 `-stats-addr`、`-web-addr` 与 `-web-log-max` 属于聚合端点的全局参数，显式设置时会覆盖实例配置。
- forward 多实例当前仅支持 `mode: go`；`mode: socat` 会在启动前返回明确错误。同一地址可分别由 TCP 与 UDP 实例监听，但相同协议与地址的重复实例会被拒绝。
- 各实例的监听地址（forward 为 `listen_host:listen_port`，proxy 为 `addr`）**不得重复**，否则启动时报错。
- 单对象写法与旧版平铺布局完全兼容，视为 1 个实例，行为不变。

命令行覆盖配置文件示例（配置文件里 `forward.listen_port` 为 22，此处显式指定 8022 生效）：

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
- TCP/UDP 目标若解析到当前 forward 监听器会在启动阶段被拒绝；TCP 建连后还会再次检查实际对端，避免 DNS 变化形成递归连接或数据包风暴。

## 可观测性

`go` 模式下：

- 每条连接在建立与关闭时各打印一行日志，包含连接序号、双方地址、上/下行字节数与连接时长。
- 维护活跃连接的原子计数。
- 在类 Unix 平台可向进程发送 `SIGUSR1` 打印当前连接快照，例如：

```bash
kill -USR1 <pid>
# 日志输出：status: active=<当前活跃连接数> total=<累计处理连接数> rejected=<拒绝数> dial-errors=<拨号失败数> up=<上行字节> down=<下行字节> uptime=<运行时长>
```

- 通过 `-stats-addr` 可另外开启一个只读 HTTP 统计端点（forward 与 proxy 均支持），例如：

```bash
./portmap -stats-addr 127.0.0.1:9090
# 或 ./portmap proxy -stats-addr 127.0.0.1:9090

curl http://127.0.0.1:9090/stats     # JSON 快照
curl http://127.0.0.1:9090/metrics   # Prometheus 文本
```

  该端点默认关闭；出于安全考虑仅允许绑定回环地址，proxy 下需显式 `-stats-allow-public` 才能绑定非回环地址。多实例（多端口映射）场景下会启动单个聚合端点，汇总所有实例的快照。

- 通过 `-web-addr` 可开启一个可选的 **Web 面板**（forward 与 proxy 均支持），在浏览器中实时查看性能统计与访问/连接日志：

```bash
./portmap -web-addr 127.0.0.1:8080
# 或 ./portmap proxy -web-addr 127.0.0.1:8080
```

  随后用浏览器打开 `http://127.0.0.1:8080/` 即可：页面顶部展示实时性能卡片（活跃/累计/拒绝连接、拨号失败、上下行字节、运行时长），下方为结构化的连接事件日志表格，并会自动轮询刷新。它是一个网页而非 `curl` 接口，但同时提供 `/api/stats` 与 `/api/logs` 两个 JSON 端点供程序化读取。

  面板默认关闭；出于安全考虑（连接日志含目标地址）仅允许绑定回环地址，proxy 下需显式 `-web-allow-public` 才能绑定非回环地址。可用 `-web-log-max` 调整连接事件环形缓冲保留的条数（默认 1000）。

- `-quiet` 抑制常规日志；`-log-level debug` 输出更详细信息（如 `pipe` 层异常）。

## 性能压测

仓库自带一个独立、自包含的压测工具 [`cmd/loadtest`](cmd/loadtest/main.go)，用于评估 portmap 转发的**可靠性**与**吞吐量**，并输出当前**主机环境**信息。

默认自包含模式下，工具会在进程内启动一个 echo 目标服务，再用 `forward.New(...)` 起一个转发服务，然后由压测客户端对转发端口发压，形成 `client -> portmap -> echo` 的完整链路，无需任何额外准备即可运行。也可用 `-external <addr>` 直接压测外部已运行的 portmap（此时需自备目标服务）。

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
  -idle-timeout duration    内建转发服务空闲超时，0 表示不启用
  -warmup duration          预热时间，预热期数据不计入统计 (默认 1s)
  -format text|json         输出格式 (默认 text)
  -max-samples int          最多保留的 RTT 样本数，使用蓄水池采样 (默认 100000)
  -max-error-rate float     允许的最大错误率百分比，设置后超限退出 1
  -max-p95 duration         允许的最大 p95 RTT，0 表示不设置阈值
```

`-requests` 是成功请求数的提前结束条件，并不会禁用 `-duration`。即使目标持续
拨号失败、超时或返回错误，压测也会在 `-duration` 到期时结束；连续失败会短暂
退避，避免不可达目标导致忙循环。

报告包含三块：**主机环境**（GOOS/GOARCH、CPU 数、Go 版本、主机名、内存分配）、**配置**、**结果**（吞吐 MB/s 与 Gbps、req/s、connrate 模式下的 conns/s、延迟 min/p50/p95/p99/max、错误率与分类计数，以及压测后 `ActiveConns()` 是否归零的可靠性校验）。RTT 使用有界蓄水池采样，长时间压测不会随请求数持续增长内存；`-format json` 提供机器可读结果。所有输出走标准库，不引入新依赖。

样例输出（节选）：

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

## 工程化

```bash
make build     # 注入版本信息编译
make test      # go test ./... -race
make vet       # go vet
make lint      # golangci-lint（需已安装）
make security  # govulncheck
make check     # 模块整洁、vet、race test、漏洞检查与构建
make release   # 交叉编译多平台产物到 dist/
make snapshot  # 用 GoReleaser 本地试跑发布流程（不推送、不发布）
```

CI 见 `.github/workflows/ci.yml`：在 Linux/macOS 上运行 `go vet` 与
`go test -race`，对全部发布平台做交叉编译，并独立运行 `golangci-lint`、
`go mod tidy -diff` 与 `govulncheck`。

## 发布

发布见 `.github/workflows/release.yml`，由 GoReleaser 驱动：

- **触发**：推送 `v*` 形式的 tag（如 `v1.2.0`）自动发布；手动
  `workflow_dispatch` 也必须从 `v*` tag 发起。
- **发布门禁**：发布前重新运行模块整洁检查、`go vet`、race test 与
  `govulncheck`。
- **二进制产物**：`linux`/`darwin` × `amd64`/`arm64`
  连同 `checksums.txt` 一并上传到 GitHub Release。
- **容器镜像**：构建多架构（`linux/amd64` + `linux/arm64`）镜像并推送到
  `ghcr.io/soulteary/portmap` 与 `docker.io/soulteary/portmap`，
  同时打 `:latest` 与 `:<version>` 标签。
- **来源证明**：GitHub 为发布归档与 `checksums.txt` 生成 Sigstore 构建证明，
  可使用 `gh attestation verify` 验证。

发布前置条件：

- ghcr.io 使用内置 `GITHUB_TOKEN`（工作流已授予 `packages: write`），无需额外配置。
- Docker Hub 需在仓库 Settings → Secrets 中配置 `DOCKERHUB_USERNAME` 与
  `DOCKERHUB_TOKEN` 两个 secret；当前发布配置同时生成 Docker Hub 镜像，缺少任一
  secret 时会在发布前明确失败，避免只发布了一部分产物。
- 本地可用 `make snapshot`（等价 `goreleaser release --snapshot --clean`）或
  `goreleaser check` 校验配置，均不会推送。

## 项目结构

```text
.
├── main.go                          # 命令行入口与参数解析
├── main_test.go                     # 命令行入口测试
├── config.go                        # YAML 配置文件加载与合并
├── config_test.go                   # 配置文件加载/合并测试
├── config.example.yaml              # 配置文件示例
├── CHANGELOG.md                     # 版本历史与升级说明
├── signals_unix.go                  # 类 Unix 平台 SIGUSR1 状态打印
├── cmd
│   └── loadtest                     # 独立压测工具（自包含链路 + TCP/UDP 压测）
│       └── main.go
├── Makefile                         # 构建/测试/发布
├── LICENSE                          # Apache 2.0 许可证
├── .golangci.yml                    # golangci-lint 配置
├── .goreleaser.yaml                 # GoReleaser 发布配置（二进制 + 镜像）
├── Dockerfile                       # scratch 基础镜像
├── .dockerignore                    # 容器构建上下文忽略规则
├── .github/workflows/ci.yml         # CI 工作流
├── .github/workflows/release.yml    # 发布工作流（GoReleaser）
└── internal
    ├── forward                      # 纯 Go TCP/UDP 转发器
    │   ├── forward.go               # TCP 转发、限流、空闲超时、日志
    │   ├── forward_test.go          # forward 单元测试
    │   ├── selftarget.go            # 递归自引用目标防护
    │   ├── udp.go                   # UDP 会话转发
    │   └── reuseaddr_unix.go        # 类 Unix 平台 SO_REUSEADDR
    ├── proxy                        # 单端口 SOCKS5 + HTTP 代理
    │   ├── server.go                # 监听、首字节协议探测、连接分发
    │   ├── socks5.go                # SOCKS5 握手与 CONNECT
    │   ├── http.go                  # HTTP 代理与 CONNECT 转发
    │   ├── dialer.go                # 忽略环境代理的直连拨号器
    │   ├── upstream.go              # SOCKS5/HTTP/SSH 上游拨号
    │   └── proxy_test.go            # proxy 单元测试
    ├── netutil                      # forward/proxy 共用的双向转发与空闲超时
    │   └── netutil.go               # Relay / RelayReader / IdleConn
    ├── stats                        # 统计、事件、JSON 与 Prometheus 输出
    ├── web                          # 只读监控面板及 JSON API
    ├── socat                        # 调用系统 socat 的 fallback
    │   ├── socat.go                 # 构造并执行 socat 命令
    │   ├── socat_test.go            # socat 单元测试
    │   └── socat_cancel_unix.go     # 类 Unix 平台 SIGTERM 优雅取消
    └── i18n                         # 多语言（i18n）支持
        ├── i18n.go                  # 语言检测、解析与查表
        ├── i18n_test.go             # i18n 单元测试
        ├── keys_common.go           # 跨命令共享消息 key + 语言表装配
        ├── keys_forward.go          # forward 子命令消息 key
        ├── keys_proxy.go            # proxy 子命令消息 key
        ├── locale_unix.go           # 类 Unix 平台区域探测（no-op）
        └── messages_*.go            # 各语言消息（en/zh/ja/ko/fr/de）
```

## 测试

```bash
go test ./...
# 或
make test
```

## 许可证

本项目基于 [Apache License 2.0](LICENSE) 开源，版权所有 (c) 2026 soulteary。
