# portmap

[![CI](https://github.com/soulteary/portmap/actions/workflows/ci.yml/badge.svg)](https://github.com/soulteary/portmap/actions/workflows/ci.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/soulteary/portmap)](https://goreportcard.com/report/github.com/soulteary/portmap) [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

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

```bash
go build -o portmap .
# or use the Makefile (injects version information automatically)
make build
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
  -idle-timeout duration  idle timeout; disconnect if no data in both directions, 0 to disable (go mode only)
  -log-level string       log level: info or debug (go mode only, default "info")
  -quiet                  quiet mode, suppress per-connection routine logs (go mode only)
  -config string          path to a YAML config file
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

## Config File

Besides command-line flags, you can also read configuration from a YAML file via `-config <path>`:

```bash
./portmap -config config.yaml
```

- **Priority**: explicitly set command-line flags > config file > built-in defaults.
  That is, only fields present in the config file and not explicitly set on the command line override the defaults.
- `dial_timeout` / `idle_timeout` are strings in YAML (e.g. `"10s"`, `"5m"`), parsed via `time.ParseDuration`.
- Unknown fields in the config file cause an error, helping catch typos early.
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
```

Example of command-line overriding the config file (config has `listen_port: 22`, here 8022 takes effect):

```bash
./portmap -config config.yaml -listen-port 8022
```

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
# log output: status: active=<current active connections> total=<cumulative connections processed>
```

  Windows has no `SIGUSR1`, so this feature is automatically skipped (without affecting compilation or execution).

- `-quiet` suppresses routine logs; `-log-level debug` outputs more detailed information (such as `pipe`-layer anomalies).

## Engineering

```bash
make build     # compile with version information injected
make test      # go test ./... -race
make vet       # go vet
make lint      # golangci-lint (must be installed)
make release   # cross-compile artifacts for multiple platforms into dist/
make snapshot  # dry-run the release flow locally via GoReleaser (no push, no publish)
```

See CI at `.github/workflows/ci.yml`: it runs `go vet` and `go test -race` on linux/macOS/windows,
and runs `golangci-lint` independently.

## Release

Releases are driven by GoReleaser, see `.github/workflows/release.yml`:

- **Trigger**: pushing a `v*` tag (e.g. `v1.0.0`) publishes automatically, or trigger
  manually via `workflow_dispatch` in the Actions tab.
- **Binaries**: `linux`/`darwin`/`windows` x `amd64`/`arm64` (except windows/arm64),
  uploaded to the GitHub Release together with `checksums.txt`.
- **Container images**: multi-arch (`linux/amd64` + `linux/arm64`) images are built and
  pushed to `ghcr.io/soulteary/portmap` and `docker.io/soulteary/portmap`, tagged with
  both `:latest` and `:<version>`.

Prerequisites:

- ghcr.io uses the built-in `GITHUB_TOKEN` (the workflow grants `packages: write`), no
  extra config needed.
- Docker Hub requires `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets configured under
  the repository Settings → Secrets.
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
    └── socat                        # fallback that invokes the system socat
        ├── socat.go                 # construct and execute the socat command
        ├── socat_test.go            # socat unit tests
        ├── socat_cancel_unix.go     # graceful SIGTERM cancellation on Unix-like platforms
        └── socat_cancel_windows.go  # Windows no-op (no SIGTERM)
```

## Testing

```bash
go test ./...
# or
make test
```

## License

This project is open-sourced under the [Apache License 2.0](LICENSE), Copyright (c) 2026 soulteary.
