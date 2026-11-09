# portmap

[![Go Test Coverage](./.github/coverage.svg)](./.github/go-test-report.md) [![Go Report Card](./.github/goreportcard.svg)](./.github/goreportcard-report.md) [![CI](https://github.com/soulteary/portmap/actions/workflows/ci.yml/badge.svg)](https://github.com/soulteary/portmap/actions/workflows/ci.yml) [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

<p align="center">
  <a href="README.en.md">ENGLISH</a> | <a href="README.md" target="_blank">中文文档</a>
</p>

<p align="center">
  <img src=".github/workflows/assets/portmap-logo.png" alt="portmap Logo" width="160"/>
</p>

`portmap` 提供 TCP/UDP 端口转发和单端口 SOCKS5/HTTP 代理。支持 Linux、macOS 的 amd64 与 arm64；默认使用 Go 实现，只有 `-mode socat` 需要系统安装 `socat`。

<p align="center">
  <img src=".github/workflows/assets/portmap-banner.jpg" alt="portmap Banner" width="720"/>
</p>

本文说明当前源码的用法。已安装版本的参数以 `portmap forward -help` 和 `portmap proxy -help` 为准；发行包可能尚未包含 `main` 分支的新功能。

- [安装](#安装)
- [本地转发](#本地转发)
- [通过 SSH 访问远端服务](#通过-ssh-访问远端服务)
- [使用 SOCKS5 或 HTTP 代理](#使用-socks5-或-http-代理)
- [SSH 认证](#ssh-认证)
- [配置文件](#配置文件)
- [可观测性](#可观测性)
- [排查问题](#排查问题)
- [性能压测](docs/performance.md)
- [开发与维护](#开发与维护)

## 安装

### Homebrew

```bash
brew tap soulteary/tap
brew install soulteary/tap/portmap
portmap version
```

已安装时可运行 `brew update` 和 `brew upgrade soulteary/tap/portmap`。也可以从 [Releases](https://github.com/soulteary/portmap/releases) 下载与系统、架构对应的归档，解压后运行其中的 `./portmap version`，或将二进制放入 PATH 中的目录。

### 从源码构建

需要 Go 1.27.3 或更新版本，具体要求见 [go.mod](go.mod)。

```bash
git clone https://github.com/soulteary/portmap.git
cd portmap
make build
./portmap version
```

不使用 `make` 时运行 `go build -o portmap .`。下文统一使用 PATH 中的 `portmap`；从源码运行时替换为 `./portmap`。

### 容器

Linux 容器镜像发布在 `ghcr.io/soulteary/portmap` 和 `soulteary/portmap`，支持 amd64、arm64。镜像基于 `scratch`，默认以用户 `65532:65532` 运行。

以下示例在原生 Linux 主机使用 host 网络，将宿主机的本地端口转发到同一主机的服务：

```bash
docker run --rm --network host ghcr.io/soulteary/portmap:latest \
  forward -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:8080
```

这里的 `127.0.0.1` 属于容器使用的网络空间。Docker Desktop 下需核对 host 网络支持及配置，不能直接假定它指向 macOS 主机。普通 bridge 网络也可以运行，但需配置端口发布，并使用容器能访问的目标地址。

需要固定版本时，在 Releases 中选择对应的镜像标签替换 `latest`。绑定低位端口是否需要权限取决于主机配置；需要时可添加 `--cap-drop ALL --cap-add NET_BIND_SERVICE --security-opt no-new-privileges:true`。

## 本地转发

假设本机已有 HTTP 服务监听 `127.0.0.1:8080`。如果只是验证用法，且已安装 Python 3，可在单独终端启动测试服务：

```bash
python3 -m http.server 8080 --bind 127.0.0.1
```

在另一个终端启动转发，保持进程运行：

```bash
portmap forward -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:8080
```

然后打开 `http://localhost:10024/`，或再开一个终端运行：

```bash
curl --noproxy '*' http://127.0.0.1:10024/
```

`-listen-port` 是本地入口，`-target` 是 portmap 要连接的服务地址。转发不会启动目标服务。

直接转发默认监听所有网卡；只供本机访问时显式设置 `-listen-host 127.0.0.1`。TCP 是默认协议；UDP 使用 `-proto udp`，并要求目标提供 UDP 服务。

```bash
portmap forward -proto udp -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:5353
```

`forward` 默认最多处理 256 个并发 TCP 连接或 UDP 会话，空闲超时为 5 分钟。`-max-conns 0` 取消数量限制；`-idle-timeout 0` 对 TCP 表示禁用空闲超时，对 UDP 则使用 60 秒会话回收时间。

不写子命令时，带参数的调用按 `forward` 处理。无参数调用只显示帮助。

```bash
portmap forward -help
portmap proxy -help
portmap version
```

在 Go 模式下，`Ctrl+C` 或 `SIGTERM` 会停止 forward 并关闭活动连接；proxy 停止接受新连接，最多等待 10 秒，再关闭未结束的连接。

## 通过 SSH 访问远端服务

此功能要求 `forward -help` 包含 `-upstream`；旧发行版没有该参数时，请从当前源码构建。

将示例中的 `user@server.example:22` 换成自己的 SSH 登录端点。在本地执行：

```bash
portmap forward \
  -listen-port 10024 \
  -target 127.0.0.1:10024 \
  -upstream ssh://user@server.example:22
```

保持运行，然后打开 `http://localhost:10024/`。浏览器不需要代理设置，也可用下面的命令验证：

```bash
curl --noproxy '*' http://127.0.0.1:10024/
```

此时 `-target` 从 SSH 服务器访问，`127.0.0.1` 指服务器自身，目标域名也由服务器解析。服务器必须允许 SSH TCP 转发，应用必须监听目标地址。本地和远端端口可以相同。

SSH 转发未指定 `-listen-host` 时默认监听本地回环地址。显式设置的地址会被保留，包括空地址和 `0.0.0.0`。此功能仅支持 `-mode go -proto tcp`；不支持 UDP、socat 或其他上游协议。SSH 失败时不会回退为直连。

SSH 连接在首次本地访问时建立。“开始监听”不代表远端认证或应用连接已经通过。多个连接复用 SSH 传输，各用独立通道；断线重连供后续连接使用，不会重放已经中断的数据流。如果 SSH 服务器就是本机，应避免让远端目标再次指向转发器。

如果 `forward -help` 没有 `-upstream`，请检查版本并从当前源码构建。认证方式见 [SSH 认证](#ssh-认证)。

## 使用 SOCKS5 或 HTTP 代理

当客户端需要选择不同目标时，使用 `proxy`。同一个端口接受 SOCKS5 和 HTTP 代理请求，HTTPS 通过 CONNECT 隧道传输。

```bash
portmap proxy -addr 127.0.0.1:1080
```

保持前面的测试 HTTP 服务运行，在另一终端验证两种协议：

```bash
curl --noproxy '' --proxy socks5h://127.0.0.1:1080 http://127.0.0.1:8080/
curl --noproxy '' --proxy http://127.0.0.1:1080 http://127.0.0.1:8080/
```

`--noproxy ''` 防止 curl 绕过回环地址。浏览器使用代理时，也需检查其本地地址绕过规则；只想直接打开一个本地端口时，使用 `forward`。

配置 SSH 上游后，目标从 SSH 服务器访问：

```bash
portmap proxy -addr 127.0.0.1:1080 -upstream ssh://user@server.example:22
```

`proxy -upstream` 还支持 `socks5://` 和 `http://` 上游。portmap 不读取 `HTTP_PROXY`、`HTTPS_PROXY`、`ALL_PROXY`、`NO_PROXY`；上游必须显式配置。IPv6 上游 URL 使用方括号，省略端口或端口为空时使用协议默认值，显式端口和 zone 标识会被保留。

代理默认监听回环地址，不提供客户端身份认证。需要监听其他地址时设置 `-allow-public`，并自行限制网络访问。`-stats-allow-public`、`-web-allow-public` 分别控制管理端点，不受 `-allow-public` 影响。

默认并发上限为 256，空闲超时为 5 分钟。`-handshake-timeout` 默认 10 秒，限制客户端协议探测和请求解析；出站连接使用独立的 `-dial-timeout`，默认 30 秒。HTTP 请求行与头合计最多 1 MiB，超限返回 `431`。普通 HTTP Upgrade 请求返回 `501`，透明升级隧道需使用 CONNECT。

直连目标不能指向代理自身监听器；配置上游时检查的是上游端点，应用目标属于上游侧地址空间。

## SSH 认证

本节同时适用于 `forward` 和 `proxy` 的 SSH 上游。默认校验主机密钥，读取 `~/.ssh/known_hosts`；可用 `-upstream-known-hosts` 指定其他文件。首次连接前，可先用普通 `ssh` 连接并核验主机指纹。`-upstream-insecure` 会跳过校验，仅用于明确需要跳过校验的测试。

### ssh-agent

portmap 默认尝试 ssh-agent。可先加入自己的私钥：

```bash
ssh-add ~/.ssh/id_ed25519
```

macOS 上可用 `ssh-add --apple-use-keychain ~/.ssh/id_ed25519` 保存口令。agent 是否能在后续登录或重启后使用，还取决于系统和 SSH 配置。

`-upstream-agent=false` 禁用 agent；`-upstream-agent-socket` 指定 Unix socket，默认读取 `SSH_AUTH_SOCK`。socket 不可达时跳过 agent。能连接 socket 不代表 agent 中的密钥会被服务器接受。

### 私钥与口令

```bash
portmap forward -listen-port 10024 -target 127.0.0.1:10024 \
  -upstream ssh://user@server.example:22 -upstream-identity ~/.ssh/id_ed25519
```

`-upstream-identity` 使用私钥，不是 `.pub` 公钥。私钥口令按以下顺序取值：`PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE` 环境变量、YAML 的 `upstream_identity_passphrase`、终端输入。没有口令且 agent 可达时跳过交互提示；加密私钥会被跳过，随后尝试 agent。

指定私钥、没有可用 agent 且未提供口令时，终端会提示输入。未加密私钥可以直接回车。非交互环境不会提示，需要提前配置认证方式。口令没有命令行参数；若保存到 YAML，应限制文件访问权限。

### 登录密码与保活

登录密码优先取自上游 URL。未配置私钥时，再读取 `PORTMAP_UPSTREAM_PASSWORD`；两者都没有、agent 不可达且终端可交互时，提示输入登录密码。配置私钥会跳过登录密码的环境变量读取和提示，但 URL 中显式提供的密码仍可与其他认证方式一起使用。

认证方法按 agent、指定私钥、密码的顺序尝试。可达的 agent 只抑制交互提示，不会使显式的私钥口令失效。

`-upstream-keepalive` 默认 30 秒，负数禁用主动保活；`-upstream-keepalive-max-failures` 默认 3。探测超时会提前回收传输。断线后重新建立 SSH 传输，已中断的应用连接需要客户端重试。

## 界面语言

帮助文本、日志与错误消息支持多语言：`en`（英文）、`zh`（简体中文）、`ja`（日文）、`ko`（韩文）、`fr`（法文）、`de`（德文），无法识别时回退英文。

默认从环境变量自动检测，检测优先级为：`PORTMAP_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` > `LANGUAGE`。也可显式覆盖：

```bash
portmap -lang zh -version          # 命令行显式指定
PORTMAP_LANG=ja portmap -version   # 环境变量指定
```

## 配置文件

除命令行 flag 外，还可通过 `-config <path>` 从 YAML 文件读取配置：

```bash
portmap forward -config config.yaml
```

- **优先级**：命令行显式设置的 flag > 配置文件 > 内置默认值。
  即：只有在配置文件中出现、且命令行未显式设置的字段，才会覆盖默认值。
- `dial_timeout` / `idle_timeout` 在 YAML 中用字符串（如 `"10s"`、`"5m"`），经 `time.ParseDuration` 解析。
- 配置文件中出现未知字段会直接报错，便于及早发现拼写错误。
- `lang` 字段仅影响运行期消息（日志/错误），**不影响** `--help` 与 flag 描述的语言：`--help` 在配置文件加载前就已定稿，如需改变其语言请用命令行 `-lang`。
- 配置文件必须包含所选子命令的非空配置；空文件或只有另一子命令的配置会报错。
- `forward` 读取 `forward:`，`proxy` 读取 `proxy:`，一次运行不会同时启动两类服务。
- 启动时会打印生效配置摘要。

将以下配置保存为 `config.yaml`。完整参数见 [config.example.yaml](config.example.yaml)：

```yaml
forward:
  listen_host: 127.0.0.1
  listen_port: 10024
  target: 127.0.0.1:8080
proxy:
  addr: 127.0.0.1:1080
```

按需运行其中一个子命令：

```bash
portmap forward -config config.yaml
# or
portmap proxy -config config.yaml
```

- **向后兼容**：旧版「平铺 forward」布局（顶层直接写 `listen_port` / `target` / … ）仍受支持，`portmap` 会自动识别并归一化到 `forward:` 段，无需迁移即可继续使用。

### 多端口映射（多实例）

`forward:` 与 `proxy:` 既可写成**单个对象**（1 个实例），也可写成**对象列表**，用于在一次运行中同时启动多个端口映射（多实例）。列表元素的字段与单对象写法完全一致：

```yaml
forward:
  - listen_host: 127.0.0.1
    listen_port: 10024
    target: 127.0.0.1:8080
  - listen_host: 127.0.0.1
    listen_port: 10025
    target: 127.0.0.1:8081

proxy:
  - addr: 127.0.0.1:1080
    upstream: ssh://user@server.example:22
  - addr: 127.0.0.1:1081
```

- 各实例并发运行；收到退出信号（`Ctrl+C` / `SIGTERM`）时按各自的关闭流程退出，任一实例发生致命错误会触发整体退出。
- 多实例**只能在配置文件中表达**；CLI 仍是单实例语义（如 `portmap proxy -addr ... -upstream ...`）。多实例场景下命令行的 per-instance flag（`-listen-port`/`-addr`/`-upstream` 等）无法一一对应，会被忽略并在日志中提示；`-config`/`-lang` 仍生效。forward 的 `-stats-addr`、`-web-addr` 与 `-web-log-max` 属于聚合端点的全局参数，显式设置时会覆盖实例配置。
- proxy 的 `-stats-addr`、`-stats-allow-public`、`-web-addr`、`-web-allow-public` 与 `-web-log-max` 同样是全局管理端点参数。
- forward 多实例当前仅支持 `mode: go`；`mode: socat` 会在启动前返回明确错误。同一地址可分别由 TCP 与 UDP 实例监听，但相同协议与地址的重复实例会被拒绝。
- proxy 实例不能使用重复的监听地址。不同实例启用的管理端点需使用一致的地址及访问设置；冲突会在启动时返回错误。
- 单对象写法与旧版平铺布局完全兼容，视为 1 个实例，行为不变。

命令行覆盖配置文件示例（配置文件里 `forward.listen_port` 为 10024，此处显式指定 10025 生效）：

```bash
portmap forward -config config.yaml -listen-port 10025
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
| SSH 上游 | 仅 TCP，使用 `-upstream ssh://…` | 不支持，配置时返回错误 |
| `SIGUSR1` 状态打印 | 支持（见下） | 不适用（由 socat 进程接管） |

- **`-listen-host` 在 socat 下的行为**：非空时会在监听地址上追加 `bind=<host>`，
  例如 `-mode socat -listen-host 127.0.0.1 -listen-port 22` 生成
  `socat TCP-LISTEN:22,bind=127.0.0.1,fork,reuseaddr TCP:127.0.0.1:2222`；
  不设置时不添加 `bind` 参数，由 socat 的默认监听行为决定。
- 在 socat 模式下显式设置了仅 `go` 模式支持的参数（`-idle-timeout`/`-max-conns`/
  `-log-level`/`-quiet`）时，程序会打印一行提示说明这些参数被忽略。

## UDP 行为说明

UDP 无连接，`go` 模式以「客户端地址 → 一条到目标的 UDP 连接」维护会话表，目标的回包按会话转发回对应客户端。以下参数在 UDP 下与 TCP 的差异：

- **`-max-conns`**：UDP 下限制**并发会话数**（而非连接数），超限时直接拒绝新客户端并丢弃其首包（UDP 无排队语义），并在 `-log-level debug` 下记录。
- **`-idle-timeout`**：UDP 下为 `0` 会**回退为默认 60s**，即回收空闲超过 60s 的会话（TCP 下 `0` 表示不启用）。
- 直连 TCP/UDP 目标若解析到当前 forward 监听器会在启动阶段被拒绝；直连 TCP 建连后还会再次检查实际对端，避免 DNS 变化形成递归连接或数据包风暴。SSH 转发的目标属于远端地址空间，不与本地监听器比较。

## 可观测性

下面的命令用于替换前面的示例；先停止占用相同监听端口的示例进程。

`go` 模式下：

- 每条连接在建立与关闭时各打印一行日志，包含连接序号、双方地址、上/下行字节数与连接时长。
- 维护活跃连接的原子计数。
- 在类 Unix 平台，先把 `PORTMAP_PID` 设置为 portmap 的实际进程号，再发送 `SIGUSR1` 打印连接快照：

```bash
kill -USR1 "$PORTMAP_PID"
# 日志输出：status: active=<当前活跃连接数> total=<累计处理连接数> rejected=<拒绝数> dial-errors=<拨号失败数> up=<上行字节> down=<下行字节> uptime=<运行时长>
```

- 通过 `-stats-addr` 可另外开启一个只读 HTTP 统计端点（forward 与 proxy 均支持），例如：

```bash
portmap forward -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:8080 -stats-addr 127.0.0.1:9090
# 或 portmap proxy -stats-addr 127.0.0.1:9090

```

在另一终端验证：

```bash
curl --noproxy '*' http://127.0.0.1:9090/stats     # JSON 快照
curl --noproxy '*' http://127.0.0.1:9090/metrics   # Prometheus 文本
```

  该端点默认关闭；出于安全考虑仅允许绑定回环地址，proxy 下需显式 `-stats-allow-public` 才能绑定非回环地址。多实例（多端口映射）场景下会启动单个聚合端点，汇总所有实例的快照。

- 通过 `-web-addr` 可开启一个可选的 **Web 面板**（forward 与 proxy 均支持），在浏览器中实时查看性能统计与访问/连接日志：

```bash
portmap forward -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:8080 -web-addr 127.0.0.1:9080
# 或 portmap proxy -web-addr 127.0.0.1:9080
```

  随后用浏览器打开 `http://127.0.0.1:9080/` 即可：页面顶部展示实时性能卡片（活跃/累计/拒绝连接、拨号失败、上下行字节、运行时长），下方为结构化的连接事件日志表格，并会自动轮询刷新。它是一个网页而非 `curl` 接口，但同时提供 `/api/stats` 与 `/api/logs` 两个 JSON 端点供程序化读取。

  面板默认关闭；出于安全考虑（连接日志含目标地址）仅允许绑定回环地址，proxy 下需显式 `-web-allow-public` 才能绑定非回环地址。可用 `-web-log-max` 调整连接事件环形缓冲保留的条数（默认 1000）。

- `-quiet` 抑制常规日志；`-log-level debug` 输出更详细信息（如 `pipe` 层异常）。


## 排查问题

### 参数不存在，或 `proxy` 没有启动

```bash
type -a portmap
portmap version
portmap forward -help
portmap proxy -help
```

确认执行的是预期路径中的二进制。旧版本可能不包含当前源码的参数或子命令；需要新功能时从当前源码构建。

### `address already in use`

本地监听端口已被占用。停止占用端口的进程，或更改 `-listen-port` / `-addr`。SSH 转发修改的是本地入口，不必同步修改远端 `-target`。Linux 可用 `ss -lntp` 查看监听，macOS 可用 `lsof -nP -iTCP:10024 -sTCP:LISTEN`。

### SSH 已连接，但目标返回 `Connection refused`

这表示 SSH 传输已经建立，应用目标拒绝了连接。在 SSH 服务器上检查应用是否启动、监听地址和端口是否正确，以及容器端口是否发布。

例如，登录远端后执行下面的命令。Linux 上还可以检查监听列表：

```bash
curl --noproxy '*' -v --connect-timeout 5 http://127.0.0.1:10024/
ss -lntp 'sport = :10024'
```

### 认证或主机密钥错误

确认 `-upstream-identity` 使用私钥。使用 agent 时用 `ssh-add -l` 检查已加载的密钥。非交互环境无法等待密码输入，应预先配置认证。主机密钥校验失败时核对目标主机指纹和 known_hosts 记录，不要把跳过校验作为常规解决办法。

### 启动后看不到页面

forward 需要有目标服务；proxy 需要客户端实际使用代理。用前面的 curl 命令区分两者，并检查浏览器的本地地址绕过规则。默认空闲超时可能关闭长期无流量的连接，需要时按协议调整 `-idle-timeout`。

## 开发与维护

- [贡献指南](CONTRIBUTING.md#贡献指南中文)：开发环境、本地检查、代码规范和发布流程。
- [性能压测](docs/performance.md)：TCP/UDP 回显压测、采样和输出参数。
- [CHANGELOG](CHANGELOG.md)：版本历史和升级影响。
- [CI 工作流](.github/workflows/ci.yml)与[发布工作流](.github/workflows/release.yml)：检查项目和发布平台的实际配置。

## 项目结构

- `main.go`、`config.go`：命令入口、参数合并和多实例启动。
- `internal/forward`：TCP/UDP 固定目标转发。
- `internal/proxy`：SOCKS5、HTTP 请求处理和连接生命周期。
- `internal/upstream`：直连、SOCKS5、HTTP CONNECT、SSH 出站传输。
- `internal/netutil`：数据复制、半关闭和空闲超时。
- `internal/stats`、`internal/web`：统计与只读监控页面。
- `internal/socat`、`internal/i18n`：socat 调用与多语言消息。
- `cmd/loadtest`：回显压测工具。

## 许可证

[Apache License 2.0](LICENSE)，Copyright (c) 2026 soulteary。
