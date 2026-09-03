# portmap

[![CI](https://github.com/soulteary/portmap/actions/workflows/ci.yml/badge.svg)](https://github.com/soulteary/portmap/actions/workflows/ci.yml) [![Go Report Card](./.github/goreportcard.svg)](.github/goreportcard-report.md) [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

<p align="center">
  <a href="README.en.md">ENGLISH</a> | <a href="README.md" target="_blank">中文文档</a>
</p>

<p align="center">
  <img src=".github/workflows/assets/portmap-logo.png" alt="portmap Logo" width="160"/>
</p>

> A lightweight **TCP/UDP port forwarding tool** — written in Go, with no dependency on the system `socat`.

<p align="center">
  <img src=".github/workflows/assets/portmap-banner.jpg" alt="portmap Banner" width="720"/>
</p>

## Overview

`portmap` is a general-purpose TCP/UDP port forwarding tool written in Go, equivalent to:

```bash
sudo socat TCP-LISTEN:22,fork,reuseaddr TCP:127.0.0.1:2222
```

It supports two modes:

- **`go` (default)**: A pure Go implementation that does not depend on the system `socat`. It is cross-platform, maps to `TCP-LISTEN`/`fork`/`reuseaddr`, and extends this with UDP, concurrency limiting, idle timeouts, and per-connection logging (see below).
- **`socat`**: Directly invokes the local `socat` command (optionally with `-sudo`), generating an equivalent command line (supports TCP/UDP).

## Why This Tool Exists

Podman (especially in rootless mode) by default does not allow unprivileged users to listen on low ports below 1024. This differs noticeably from Docker's port mapping behavior: in Docker you can map a container directly to low host ports such as 22/53/80, whereas under rootless Podman the same mapping fails due to permission restrictions.

A common workaround is to tweak system settings, e.g. lowering `sysctl net.ipv4.ip_unprivileged_port_start`, but this modifies a global kernel parameter, affects the security boundary of the entire machine, and is not always desirable.

`portmap` provides a more lightweight path: let the container listen on a high port (e.g. 2222) in rootless mode, then use this tool on the host to forward a low port (e.g. 22/53/80) to that high port. This preserves the Docker-like port mapping experience without adjusting system-level settings such as `net.ipv4.ip_unprivileged_port_start`.

Podman example: a rootless container listens on `2222`, and `portmap` exposes host port 22 (listening on 22 requires privileges, hence `sudo`):

```bash
# rootless container (example): map the service to the high port 2222
podman run -d -p 2222:22 your-image

# use portmap to forward the low port 22 to the container's 2222
sudo portmap -listen-port 22 -target 127.0.0.1:2222
```

## Build

Requires Go 1.27+ (the exact version in `go.mod` is the source of truth).

```bash
go build -o portmap .
# or use the Makefile (injects version information automatically)
make build
```

## Homebrew

On macOS / Linux you can install via the author's [Homebrew Tap](https://github.com/soulteary/homebrew-tap):

```bash
brew tap soulteary/tap
brew install soulteary/tap/portmap
```

Verify:

```bash
portmap --version
# portmap 1.1.0 (commit 85fc65e, built 2026-08-08T10:26:23Z)
```

## Container Image

Prebuilt multi-arch images (`linux/amd64` + `linux/arm64`) are published to ghcr.io and Docker Hub:

```bash
# pull from ghcr.io
docker pull ghcr.io/soulteary/portmap:latest
# or pull from Docker Hub
docker pull soulteary/portmap:latest
```

The image is based on `scratch` and contains only the single static binary. Use host networking so port forwarding works:

```bash
# forward host port 22 to the container listening on 2222 (low ports need privileges)
docker run --rm --network host ghcr.io/soulteary/portmap:latest \
  -listen-port 22 -target 127.0.0.1:2222
```

## Usage

```text
portmap [flags]

flags:
  -listen-port int        local listening port (default 22)
  -listen-host string     local listening address (default: all interfaces)
  -target string          forwarding target address host:port (default "127.0.0.1:2222")
  -mode string            forwarding mode: go or socat (default "go")
  -proto string           forwarding protocol: tcp or udp (default "tcp")
  -reuseaddr              enable SO_REUSEADDR (default true)
  -sudo                   run socat with sudo in socat mode
  -dial-timeout duration  dial target timeout (default 10s)
  -max-conns int          max concurrent connections, 0 for unlimited (go mode only)
  -idle-timeout duration  idle timeout; reclaim a direction once it stays idle past the threshold, 0 to disable (go mode only)
  -log-level string       log level: info or debug (go mode only, default "info")
  -quiet                  quiet mode, suppress per-connection routine logs (go mode only)
  -stats-addr string      optional HTTP stats endpoint address (e.g. 127.0.0.1:9090), empty disables; loopback only
  -web-addr string        optional Web panel listen address (e.g. 127.0.0.1:8080), empty disables; loopback only
  -web-log-max int        max number of connection events kept in the Web panel ring buffer (default 1000)
  -config string          path to a YAML config file
  -lang string            interface language: en/zh/ja/ko/fr/de (auto-detected from the system by default)
  -version                print version information and exit
```

## Examples

Fully equivalent to the original command (requires root to listen on port 22):

```bash
sudo ./portmap -listen-port 22 -target 127.0.0.1:2222
```

Reuse the system `socat` (generate the equivalent command line):

```bash
./portmap -mode socat -sudo -listen-port 22 -target 127.0.0.1:2222
```

UDP forwarding (e.g. DNS):

```bash
./portmap -proto udp -listen-port 53 -target 127.0.0.1:5353
```

Local testing on a high port + concurrency limiting + idle timeout + debug logging:

```bash
./portmap -listen-port 13000 -target 127.0.0.1:2222 \
  -max-conns 100 -idle-timeout 5m -log-level debug
```

Check the version:

```bash
./portmap -version
```

Press `Ctrl+C` to exit gracefully; it waits for in-flight connections to finish.

## SOCKS5 + HTTP Proxy

The `proxy` subcommand serves SOCKS5 and HTTP/HTTPS proxy clients on one port.
Outbound connections are direct and ignore environment proxy variables.

```bash
./portmap proxy -addr 127.0.0.1:1080
```

Important proxy flags:

- `-max-conns 256`: bound concurrent client connections (`0` disables the limit).
- `-handshake-timeout 10s`: bound protocol detection and handshake time.
- `-idle-timeout 5m`: close tunnels that remain idle in either direction.
- `-allow-public`: explicitly allow a non-loopback listen address.
- `-stats-addr 127.0.0.1:9090`: optional read-only HTTP stats endpoint (empty disables; loopback only unless `-allow-public`).
- `-web-addr 127.0.0.1:8080`: optional Web panel (empty disables; loopback only unless `-allow-public`).
- `-web-log-max 1000`: max number of connection events kept in the Web panel ring buffer.

The proxy has no authentication, so it rejects non-loopback listen addresses by
default. Only use `-allow-public` behind an appropriate firewall or equivalent
access boundary. Requests that resolve back to the proxy's own listener are also
rejected to prevent recursive connection storms. On shutdown, existing connections
have up to 10 seconds to finish before they are closed.

#### SSH upstream private key passphrase

When `upstream` is `ssh://` and the private key given via `-upstream-identity` is
**encrypted**, a passphrase is required to decrypt it. The passphrase is resolved
in the following priority order (highest → lowest):

1. the `PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE` environment variable;
2. the `upstream_identity_passphrase` config field;
3. an interactive terminal prompt (only when the two above are empty,
   `-upstream-identity` is set, and stdin is a TTY; input is read without echo).

For security there is **no command-line flag** (to avoid leaking the passphrase
into the process list or shell history). Prefer the **environment variable**; if
you store it as plaintext in YAML, control the config file permissions yourself.
The passphrase never appears in logs. When the key is encrypted but no passphrase
is provided, a clear error is returned.

```bash
# Environment variable (recommended)
export PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE='your-passphrase'
./portmap proxy -upstream ssh://root@host:22 -upstream-identity ~/.ssh/id_rsa
```

## Interface Language

Help text, logs, and error messages are localized: `en` (English), `zh` (Simplified Chinese), `ja` (Japanese), `ko` (Korean), `fr` (French), `de` (German), falling back to English when unrecognized.

The language is auto-detected from the system locale by default, with the following precedence: `PORTMAP_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` > `LANGUAGE` > system locale (on Windows). You can also override it explicitly:

```bash
./portmap -lang zh -version          # via command-line flag
PORTMAP_LANG=ja ./portmap -version   # via environment variable
```

## Config File

Besides command-line flags, you can also read configuration from a YAML file via `-config <path>`:

```bash
./portmap -config config.yaml
```

- **Priority**: explicitly set command-line flags > config file > built-in defaults.
  That is, only fields present in the config file and not explicitly set on the command line override the defaults.
- `dial_timeout` / `idle_timeout` are strings in YAML (e.g. `"10s"`, `"5m"`), parsed via `time.ParseDuration`.
- Unknown fields in the config file cause an error, helping catch typos early.
- The `lang` field only affects runtime messages (logs/errors); it does **not** change the language of `--help` or flag descriptions, because `--help` is finalized before the config file is loaded. Use the command-line `-lang` if you need to change the `--help` language.
- On startup a one-line "effective config" summary is printed to confirm the merged parameters.

See the full example in [config.example.yaml](config.example.yaml):

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
stats_addr: ""
web_addr: ""
web_log_max: 1000
lang: en
```

Example of command-line overriding the config file (config has `listen_port: 22`, here 8022 takes effect):

```bash
./portmap -config config.yaml -listen-port 8022
```

### Multiple Port Mappings (multi-instance)

`forward:` and `proxy:` may be written either as a **single object** (one instance) or as a **list of objects**, to start multiple port mappings in a single run (multi-instance). List elements use exactly the same fields as the single-object form:

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

- Each instance starts concurrently in its own goroutine; on a shutdown signal (`Ctrl-C` / `SIGTERM`) they all shut down gracefully, and a fatal error in any instance triggers overall shutdown.
- Multi-instance can **only be expressed in the config file**; the CLI stays single-instance (e.g. `portmap proxy -addr ... -upstream ...`). In the multi-instance case, per-instance CLI flags (`-listen-port`/`-addr`/`-upstream`, etc.) cannot map to a specific instance and are ignored with a notice; `-config`/`-lang` still take effect.
- The listen address of each instance (`listen_host:listen_port` for forward, `addr` for proxy) must **not be duplicated**, otherwise startup fails.
- The single-object form and the legacy flat layout remain fully compatible, treated as one instance with unchanged behavior.

## Mode Differences (go vs socat)

The two modes support different capabilities; choose based on your needs:

| Flag / Capability | `go` mode | `socat` mode |
| --- | --- | --- |
| `-listen-host` | Supported (bind to a specific address) | Supported (generates `bind=<host>`) |
| `-proto tcp/udp` | Supported | Supported |
| `-reuseaddr` | Supported | Supported (`reuseaddr`) |
| `-max-conns` | Supported | **Not supported** (ignored with a notice) |
| `-idle-timeout` | Supported | **Not supported** (ignored with a notice) |
| `-log-level` | Supported | **Not supported** (ignored with a notice) |
| `-quiet` | Supported | **Not supported** (ignored with a notice) |
| `SIGUSR1` status print | Supported (see below) | Not applicable (handled by the socat process) |

- **Behavior of `-listen-host` under socat**: when non-empty, `bind=<host>` is appended to the listen address,
  e.g. `-mode socat -listen-host 127.0.0.1 -listen-port 22` generates
  `socat TCP-LISTEN:22,bind=127.0.0.1,fork,reuseaddr TCP:127.0.0.1:2222`;
  when unset, the behavior is identical to before (listens on the port only, equivalent to `0.0.0.0`).
- In socat mode, if you explicitly set flags supported only by `go` mode (`-idle-timeout`/`-max-conns`/
  `-log-level`/`-quiet`), the program prints a line noting that these flags are ignored.

## UDP Behavior

UDP is connectionless. In `go` mode, sessions are maintained as "client address → one UDP connection to the target", and replies from the target are forwarded back to the corresponding client per session. The following flags differ from TCP under UDP:

- **`-max-conns`**: under UDP this limits the **number of concurrent sessions** (rather than connections). When exceeded, new clients are rejected directly and their first packet is dropped (UDP has no queuing semantics), which is logged under `-log-level debug`.
- **`-idle-timeout`**: under UDP a value of `0` **falls back to the default 60s**, i.e. sessions idle for more than 60s are reclaimed (under TCP, `0` means disabled).

## Observability

In `go` mode:

- Each connection prints one log line on establishment and one on closing, including the connection sequence number, both endpoints' addresses, upload/download byte counts, and connection duration.
- Maintains an atomic counter of active connections.
- On Unix-like platforms you can send `SIGUSR1` to the process to print a snapshot of current connections, e.g.:

```bash
kill -USR1 <pid>
# log output: status: active=<current active connections> total=<cumulative connections> rejected=<rejected> dial-errors=<dial failures> up=<upload bytes> down=<download bytes> uptime=<uptime>
```

  Windows has no `SIGUSR1`, so this feature is automatically skipped (without affecting compilation or execution).

- With `-stats-addr` you can additionally expose a read-only HTTP stats endpoint (both `forward` and `proxy`), e.g.:

```bash
./portmap -stats-addr 127.0.0.1:9090
# or ./portmap proxy -stats-addr 127.0.0.1:9090

curl http://127.0.0.1:9090/stats     # JSON snapshot
curl http://127.0.0.1:9090/metrics   # Prometheus text
```

  The endpoint is disabled by default and, for safety, only binds to loopback addresses; under `proxy` you must pass `-allow-public` to bind a non-loopback address. In multi-instance (multi-mapping) setups a single aggregated endpoint is started, summing the snapshots of all instances.

- With `-web-addr` you can enable an optional **Web panel** (both `forward` and `proxy`) to view live performance stats and access/connection logs in your browser:

```bash
./portmap -web-addr 127.0.0.1:8080
# or ./portmap proxy -web-addr 127.0.0.1:8080
```

  Then open `http://127.0.0.1:8080/` in a browser: the top of the page shows live performance cards (active/total/rejected connections, dial errors, up/down bytes, uptime), followed by a structured connection-event log table, all refreshed by automatic polling. It is a browser page rather than a `curl` endpoint, but it also exposes `/api/stats` and `/api/logs` JSON endpoints for programmatic access.

  The panel is disabled by default and, for safety (connection logs include target addresses), only binds to loopback addresses; under `proxy` you must pass `-allow-public` to bind a non-loopback address. Use `-web-log-max` to tune how many connection events the ring buffer keeps (default 1000).

- `-quiet` suppresses routine logs; `-log-level debug` outputs more detailed information (such as `pipe`-layer anomalies).

## Performance / Stress Testing

The repository ships a standalone, self-contained load-test tool at [`cmd/loadtest`](cmd/loadtest/main.go) to evaluate the **reliability** and **throughput** of portmap forwarding, and to report the current **host environment**.

In the default self-contained mode the tool starts an in-process echo target, brings up a forwarder via `forward.New(...)`, and then has the load client push traffic at the forwarding port, forming a full `client -> portmap -> echo` chain that runs with zero setup. Use `-external <addr>` to instead stress an already-running portmap (you must provide your own target service in that case).

```bash
# TCP throughput mode (long-lived connections, continuous echo)
go run ./cmd/loadtest -proto tcp -mode throughput -conns 100 -duration 10s

# TCP connection-rate mode (short connection open/close loop)
go run ./cmd/loadtest -proto tcp -mode connrate -conns 100 -duration 10s

# UDP throughput mode
go run ./cmd/loadtest -proto udp -mode throughput -conns 50 -duration 10s -payload 512

# Up to 1000 successful requests per connection, with a 30s hard deadline
go run ./cmd/loadtest -proto tcp -conns 20 -requests 1000 -duration 30s -max-conns 200 -idle-timeout 5m

# Stress an externally running portmap (bring your own target)
go run ./cmd/loadtest -external 127.0.0.1:13000 -proto tcp -duration 10s
```

Flags:

```text
loadtest [flags]

flags:
  -proto tcp|udp            forwarding protocol (default tcp)
  -conns int                concurrent connections/sessions (default 50)
  -duration duration        maximum runtime and hard deadline for every mode (default 10s)
  -requests int             successful requests per connection; 0 means duration-only (default 0)
  -payload int              payload bytes per request (default 1024)
  -mode throughput|connrate throughput (persistent conns) or connrate (short-conn loop) (default throughput)
  -external string          external portmap address; empty means build the chain in-process
  -max-conns int            built-in forwarder max concurrent connections, 0 = unlimited
  -idle-timeout duration    built-in forwarder idle timeout, 0 = disabled
  -warmup duration          warmup period; data during warmup is excluded from stats (default 1s)
```

`-requests` is an early completion condition based on successful requests; it does
not disable `-duration`. The run therefore ends at the duration deadline even when
the target continuously fails to dial, times out, or returns errors. Repeated
failures also use a short backoff to avoid a busy loop against an unreachable target.

The report has three blocks: **Host / Runtime** (GOOS/GOARCH, CPU count, Go version, hostname, memory allocation), **Config**, and **Results** (throughput in MB/s and Gbps, req/s, conns/s in connrate mode, latency min/p50/p95/p99/max, error rate with per-category counts, and a reliability check that `ActiveConns()` returned to zero after the run). All output uses only the standard library; no new dependencies.

Sample output (excerpt):

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

## Engineering

```bash
make build     # compile with version information injected
make test      # go test ./... -race
make vet       # go vet
make lint      # golangci-lint (must be installed)
make security  # govulncheck
make check     # module hygiene, vet, race test, vulnerability scan, and build
make release   # cross-compile artifacts for multiple platforms into dist/
make snapshot  # dry-run the release flow locally via GoReleaser (no push, no publish)
```

See CI at `.github/workflows/ci.yml`: it runs `go vet` and `go test -race` on
linux/macOS/windows, cross-builds every release target, and independently runs
`golangci-lint`, `go mod tidy -diff`, and `govulncheck`.

## Release

Releases are driven by GoReleaser, see `.github/workflows/release.yml`:

- **Trigger**: pushing a `v*` tag (e.g. `v1.0.0`) publishes automatically; a manual
  `workflow_dispatch` must also be run from a `v*` tag.
- **Release gate**: race tests, module hygiene, and `govulncheck` run again before publishing.
- **Binaries**: `linux`/`darwin`/`windows` x `amd64`/`arm64`,
  uploaded to the GitHub Release together with `checksums.txt`.
- **Container images**: multi-arch (`linux/amd64` + `linux/arm64`) images are built and
  pushed to `ghcr.io/soulteary/portmap` and `docker.io/soulteary/portmap`, tagged with
  both `:latest` and `:<version>`.
- **Provenance**: GitHub generates Sigstore build attestations for release archives and
  `checksums.txt`, verifiable with `gh attestation verify`.

Prerequisites:

- ghcr.io uses the built-in `GITHUB_TOKEN` (the workflow grants `packages: write`), no
  extra config needed.
- Docker Hub requires `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets configured under
  the repository Settings → Secrets. The release config always builds Docker Hub images,
  so a missing secret now fails clearly before publishing instead of producing a partial release.
- Locally you can validate the config with `make snapshot`
  (equivalent to `goreleaser release --snapshot --clean`) or `goreleaser check`; neither pushes anything.

## Project Structure

```text
.
├── main.go                          # command-line entry point and flag parsing
├── main_test.go                     # command-line entry point tests
├── config.go                        # YAML config file loading and merging
├── config_test.go                   # config file loading/merging tests
├── config.example.yaml              # config file example
├── signals_unix.go                  # SIGUSR1 status print on Unix-like platforms
├── signals_windows.go               # Windows no-op (no SIGUSR1)
├── cmd
│   └── loadtest                     # standalone load-test tool (self-contained chain + TCP/UDP stress)
│       └── main.go
├── Makefile                         # build/test/release
├── LICENSE                          # Apache 2.0 license
├── .golangci.yml                    # golangci-lint configuration
├── .goreleaser.yaml                 # GoReleaser release config (binaries + images)
├── Dockerfile                       # scratch base image
├── .dockerignore                    # container build context ignore rules
├── .github/workflows/ci.yml         # CI workflow
├── .github/workflows/release.yml    # release workflow (GoReleaser)
└── internal
    ├── forward                      # pure Go TCP/UDP forwarder
    │   ├── forward.go               # TCP forwarding, limiting, idle timeout, logging
    │   ├── forward_test.go          # forward unit tests
    │   ├── udp.go                   # UDP session forwarding
    │   ├── reuseaddr_unix.go        # SO_REUSEADDR on Unix-like platforms
    │   └── reuseaddr_windows.go     # SO_REUSEADDR on Windows
    ├── socat                        # fallback that invokes the system socat
        ├── socat.go                 # construct and execute the socat command
        ├── socat_test.go            # socat unit tests
        ├── socat_cancel_unix.go     # graceful SIGTERM cancellation on Unix-like platforms
        └── socat_cancel_windows.go  # Windows no-op (no SIGTERM)
    └── i18n                         # internationalization (i18n) support
        ├── i18n.go                  # language detection, parsing, and lookup
        ├── i18n_test.go             # i18n unit tests
        ├── keys.go                  # message key constants and language tables
        ├── locale_unix.go           # locale probing on Unix-like platforms (no-op)
        ├── locale_windows.go        # locale probing on Windows
        └── messages_*.go            # per-language messages (en/zh/ja/ko/fr/de)
```

## Testing

```bash
go test ./...
# or
make test
```

## License

This project is open-sourced under the [Apache License 2.0](LICENSE), Copyright (c) 2026 soulteary.
