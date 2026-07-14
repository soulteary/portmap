# portmap

English | [中文](README.md)

A general-purpose TCP/UDP port forwarding tool written in Go, equivalent to:

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
```

See CI at `.github/workflows/ci.yml`: it runs `go vet` and `go test -race` on linux/macOS/windows,
and runs `golangci-lint` independently.

## Project Structure

```text
.
├── main.go                          # command-line entry point and flag parsing
├── main_test.go                     # command-line entry point tests
├── signals_unix.go                  # SIGUSR1 status print on Unix-like platforms
├── signals_windows.go               # Windows no-op (no SIGUSR1)
├── Makefile                         # build/test/release
├── LICENSE                          # Apache 2.0 license
├── .golangci.yml                    # golangci-lint configuration
├── .github/workflows/ci.yml         # CI workflow
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
