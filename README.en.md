# portmap

[![Go Test Coverage](./.github/coverage.svg)](./.github/go-test-report.md) [![Go Report Card](./.github/goreportcard.svg)](./.github/goreportcard-report.md) [![CI](https://github.com/soulteary/portmap/actions/workflows/ci.yml/badge.svg)](https://github.com/soulteary/portmap/actions/workflows/ci.yml) [![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

<p align="center">
  <a href="README.en.md">ENGLISH</a> | <a href="README.md" target="_blank">中文文档</a>
</p>

<p align="center">
  <img src=".github/workflows/assets/portmap-logo.png" alt="portmap Logo" width="160"/>
</p>

`portmap` provides TCP/UDP port forwarding and a single-port SOCKS5/HTTP proxy. It supports Linux and macOS on amd64 and arm64. The default Go mode needs no system `socat`; only `-mode socat` requires it.

<p align="center">
  <img src=".github/workflows/assets/portmap-banner.jpg" alt="portmap Banner" width="720"/>
</p>

This guide describes the current source. Check `portmap forward -help` and `portmap proxy -help` for the installed binary; published packages may not include every feature on `main`.

- [Install](#install)
- [Forward a local port](#forward-a-local-port)
- [Access a remote service through SSH](#access-a-remote-service-through-ssh)
- [Use SOCKS5 or HTTP proxying](#use-socks5-or-http-proxying)
- [SSH authentication](#ssh-authentication)
- [Config file](#config-file)
- [Observability](#observability)
- [Troubleshooting](#troubleshooting)
- [Performance testing](docs/performance.en.md)
- [Development and maintenance](#development-and-maintenance)

## Install

### Homebrew

```bash
brew tap soulteary/tap
brew install soulteary/tap/portmap
portmap version
```

For an existing installation, run `brew update` and `brew upgrade soulteary/tap/portmap`. Alternatively, download an archive for your OS and architecture from [Releases](https://github.com/soulteary/portmap/releases). Extract it and run `./portmap version`, or put the binary in a directory on PATH.

### Build from source

Requires Go 1.27.3 or newer; [go.mod](go.mod) records the requirement.

```bash
git clone https://github.com/soulteary/portmap.git
cd portmap
make build
./portmap version
```

Without `make`, use `go build -o portmap .`. Commands below assume `portmap` is on PATH; use `./portmap` when running the local build.

### Containers

Linux images are published at `ghcr.io/soulteary/portmap` and `soulteary/portmap` for amd64 and arm64. The `scratch` image runs as user `65532:65532` by default.

This example uses host networking on a native Linux host to forward a host-local port to another service on that host:

```bash
docker run --rm --network host ghcr.io/soulteary/portmap:latest \
  forward -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:8080
```

Here, `127.0.0.1` belongs to the container's network namespace. For Docker Desktop, check its host-network support and configuration rather than assuming this address reaches the macOS host. Bridge networking also works when ports are published and the target is reachable from the container.

To pin an image, choose a release tag and replace `latest`. Whether low ports need extra privileges depends on the host configuration; when needed, add `--cap-drop ALL --cap-add NET_BIND_SERVICE --security-opt no-new-privileges:true`.

## Forward a local port

Assume an HTTP service is listening on `127.0.0.1:8080`. For a local test, if Python 3 is installed, start a test service in a separate terminal:

```bash
python3 -m http.server 8080 --bind 127.0.0.1
```

Start the forwarder in another terminal and leave it running:

```bash
portmap forward -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:8080
```

Open `http://localhost:10024/`, or run from another terminal:

```bash
curl --noproxy '*' http://127.0.0.1:10024/
```

`-listen-port` is the local entry point; `-target` is the service portmap connects to. Forwarding does not start the target service.

Direct forwarding listens on all interfaces by default. Set `-listen-host 127.0.0.1` for local-only access. TCP is the default; use `-proto udp` for a UDP target:

```bash
portmap forward -proto udp -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:5353
```

The default limit is 256 concurrent TCP connections or UDP sessions, with a 5-minute idle timeout. `-max-conns 0` removes the count limit. `-idle-timeout 0` disables the TCP idle timeout but uses a 60-second session timeout for UDP.

A call with flags but no subcommand selects `forward`. A call with no arguments only displays help.

```bash
portmap forward -help
portmap proxy -help
portmap version
```

In Go mode, `Ctrl+C` or `SIGTERM` stops forward and closes active connections. Proxy stops accepting clients, waits up to 10 seconds, then closes remaining connections.

## Access a remote service through SSH

This requires `-upstream` in `forward -help`. If an older release lacks that flag, build the current source.

Replace `user@server.example:22` with your SSH login endpoint. Run locally:

```bash
portmap forward \
  -listen-port 10024 \
  -target 127.0.0.1:10024 \
  -upstream ssh://user@server.example:22
```

Leave it running and open `http://localhost:10024/`. No browser proxy settings are needed. To test from a terminal:

```bash
curl --noproxy '*' http://127.0.0.1:10024/
```

The SSH server connects to `-target`: its loopback address refers to that server, and it resolves target hostnames. SSH TCP forwarding must be allowed, and the application must be listening on the target address. Local and remote port numbers can match.

When `-listen-host` is omitted, SSH forwarding binds to local loopback. Explicit addresses, including an empty address or `0.0.0.0`, are honored. This requires `-mode go -proto tcp`; UDP, socat, and non-SSH upstreams are rejected. SSH failure never falls back to a direct connection.

SSH connects on the first local request. A listening message does not confirm remote authentication or target availability. Clients share an SSH transport with separate channels. Reconnection serves later connections; interrupted application streams are not replayed. If the SSH server is the local machine, avoid pointing the remote target back at this forwarder.

If `forward -help` has no `-upstream`, check the version and build from the current source. See [SSH authentication](#ssh-authentication) for credentials.

## Use SOCKS5 or HTTP proxying

Use `proxy` when clients need to choose different targets. One port accepts SOCKS5 and HTTP proxy requests; HTTPS uses CONNECT tunnels.

```bash
portmap proxy -addr 127.0.0.1:1080
```

With the earlier test HTTP service running, verify both protocols in another terminal:

```bash
curl --noproxy '' --proxy socks5h://127.0.0.1:1080 http://127.0.0.1:8080/
curl --noproxy '' --proxy http://127.0.0.1:1080 http://127.0.0.1:8080/
```

`--noproxy ''` prevents curl from bypassing loopback addresses. Check local-address bypass rules when configuring a browser. For direct access to a fixed local port, use `forward`.

With an SSH upstream, targets are reached from the SSH server:

```bash
portmap proxy -addr 127.0.0.1:1080 -upstream ssh://user@server.example:22
```

`proxy -upstream` also supports `socks5://` and `http://`. portmap ignores `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY`, and `NO_PROXY`; configure the upstream explicitly. IPv6 upstream URLs use brackets. Missing or empty ports receive scheme defaults; explicit ports and zone identifiers are preserved.

The proxy binds to loopback by default and does not authenticate clients. Use `-allow-public` for another listen address and restrict network access separately. `-stats-allow-public` and `-web-allow-public` control admin endpoints independently of `-allow-public`.

The default concurrency limit is 256 and the idle timeout is 5 minutes. `-handshake-timeout` defaults to 10 seconds for client protocol detection and request parsing. Outbound setup uses a separate `-dial-timeout`, defaulting to 30 seconds. HTTP request lines and headers are limited to 1 MiB, with `431` on overflow. Plain HTTP Upgrade requests return `501`; use CONNECT for a transparent upgraded tunnel.

Direct targets cannot point at this proxy's own listener. With an upstream, checks apply to the upstream endpoint; application targets belong to the upstream's namespace.

## SSH authentication

These settings apply to SSH upstreams for both `forward` and `proxy`. Host-key verification is enabled by default using `~/.ssh/known_hosts`; use `-upstream-known-hosts` for another file. You can connect with ordinary `ssh` first and verify the host fingerprint. `-upstream-insecure` skips verification and is intended for tests that explicitly need that behavior.

### ssh-agent

portmap attempts ssh-agent authentication by default. Add your private key first:

```bash
ssh-add ~/.ssh/id_ed25519
```

On macOS, `ssh-add --apple-use-keychain ~/.ssh/id_ed25519` can store the passphrase. Availability after later logins or reboots also depends on the system and SSH configuration.

`-upstream-agent=false` disables the agent. `-upstream-agent-socket` selects a Unix socket, defaulting to `SSH_AUTH_SOCK`. An unreachable socket skips agent authentication. A reachable socket does not guarantee that the server accepts any key loaded in the agent.

### Private keys and passphrases

```bash
portmap forward -listen-port 10024 -target 127.0.0.1:10024 \
  -upstream ssh://user@server.example:22 -upstream-identity ~/.ssh/id_ed25519
```

`-upstream-identity` takes a private key, not a `.pub` public key. Passphrase precedence is: `PORTMAP_UPSTREAM_IDENTITY_PASSPHRASE`, YAML `upstream_identity_passphrase`, then a terminal prompt. A reachable agent suppresses the prompt; an encrypted key without a passphrase is skipped so agent authentication can be attempted.

A configured identity prompts when no passphrase or reachable agent is available. For an unencrypted key, press Enter. Non-interactive processes do not prompt and need credentials configured in advance. There is no passphrase flag; restrict file access if storing it in YAML.

### Login passwords and keepalives

A password in the upstream URL takes precedence. Without an identity file, portmap then reads `PORTMAP_UPSTREAM_PASSWORD`. If neither is supplied, no agent is reachable, and stdin is a terminal, it prompts for the login password. An identity file suppresses both environment-password lookup and that prompt, but an explicit URL password can still be used with other authentication methods.

Methods are attempted in agent, identity, password order. A reachable agent suppresses interactive prompts without overriding an explicitly supplied key passphrase.

`-upstream-keepalive` defaults to 30 seconds; negative values disable active probes. `-upstream-keepalive-max-failures` defaults to 3. A probe timeout can retire the transport earlier. Reconnection establishes a new transport; clients must retry interrupted application connections.

## Interface Language

Help text, logs, and error messages are localized: `en` (English), `zh` (Simplified Chinese), `ja` (Japanese), `ko` (Korean), `fr` (French), `de` (German), falling back to English when unrecognized.

The language is auto-detected from environment variables by default, with the following precedence: `PORTMAP_LANG` > `LC_ALL` > `LC_MESSAGES` > `LANG` > `LANGUAGE`. You can also override it explicitly:

```bash
portmap -lang zh -version          # via command-line flag
PORTMAP_LANG=ja portmap -version   # via environment variable
```

## Config File

Besides command-line flags, you can also read configuration from a YAML file via `-config <path>`:

```bash
portmap forward -config config.yaml
```

- **Priority**: explicitly set command-line flags > config file > built-in defaults.
  That is, only fields present in the config file and not explicitly set on the command line override the defaults.
- `dial_timeout` / `idle_timeout` are strings in YAML (e.g. `"10s"`, `"5m"`), parsed via `time.ParseDuration`.
- Unknown fields in the config file cause an error, helping catch typos early.
- The `lang` field only affects runtime messages (logs/errors); it does **not** change the language of `--help` or flag descriptions, because `--help` is finalized before the config file is loaded. Use the command-line `-lang` if you need to change the `--help` language.
- The file must contain a nonempty configuration for the selected subcommand. An empty file or a file containing only the other subcommand is rejected.
- `forward` reads `forward:` and `proxy` reads `proxy:`; a single invocation does not start both kinds of service.
- Startup logs summarize the effective configuration.

Save this as `config.yaml`. See [config.example.yaml](config.example.yaml) for the full set of fields:

```yaml
forward:
  listen_host: 127.0.0.1
  listen_port: 10024
  target: 127.0.0.1:8080
proxy:
  addr: 127.0.0.1:1080
```

Run the selected subcommand:

```bash
portmap forward -config config.yaml
# or
portmap proxy -config config.yaml
```

Example of command-line overriding the config file (config has `forward.listen_port: 10024`, here 10025 takes effect):

```bash
portmap forward -config config.yaml -listen-port 10025
```

The legacy flat forward layout remains supported. New files can use the nested sections shown above.

### Multiple Port Mappings (multi-instance)

`forward:` and `proxy:` may be written either as a **single object** (one instance) or as a **list of objects**, to start multiple port mappings in a single run (multi-instance). List elements use exactly the same fields as the single-object form:

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

- Instances run concurrently and follow their shutdown procedures on `Ctrl+C` or `SIGTERM`. A fatal error in any instance triggers overall shutdown.
- Multi-instance can **only be expressed in the config file**; the CLI stays single-instance (e.g. `portmap proxy -addr ... -upstream ...`). In the multi-instance case, per-instance CLI flags (`-listen-port`/`-addr`/`-upstream`, etc.) cannot map to a specific instance and are ignored with a notice; `-config`/`-lang` still take effect. For forward, `-stats-addr`, `-web-addr`, and `-web-log-max` are global aggregate-endpoint options and override instance values when explicitly set.
- Proxy admin flags (`-stats-addr`, `-stats-allow-public`, `-web-addr`, `-web-allow-public`, `-web-log-max`) are also global aggregate-endpoint overrides.
- Forward multi-instance currently supports only `mode: go`; `mode: socat` fails explicitly before startup. TCP and UDP instances may share an address, while duplicate instances using the same protocol and address are rejected.
- Proxy instances cannot use duplicate listen addresses. Instances enabling admin endpoints must agree on their addresses and access settings; conflicts fail at startup.
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
| SSH upstream | TCP only, via `-upstream ssh://…` | Rejected |
| `SIGUSR1` status print | Supported (see below) | Not applicable (handled by the socat process) |

- **Behavior of `-listen-host` under socat**: when non-empty, `bind=<host>` is appended to the listen address,
  e.g. `-mode socat -listen-host 127.0.0.1 -listen-port 22` generates
  `socat TCP-LISTEN:22,bind=127.0.0.1,fork,reuseaddr TCP:127.0.0.1:2222`;
  when unset, no `bind` argument is added and socat chooses its default listen behavior.
- In socat mode, if you explicitly set flags supported only by `go` mode (`-idle-timeout`/`-max-conns`/
  `-log-level`/`-quiet`), the program prints a line noting that these flags are ignored.

## UDP Behavior

UDP is connectionless. In `go` mode, sessions are maintained as "client address → one UDP connection to the target", and replies from the target are forwarded back to the corresponding client per session. The following flags differ from TCP under UDP:

- **`-max-conns`**: under UDP this limits the **number of concurrent sessions** (rather than connections). When exceeded, new clients are rejected directly and their first packet is dropped (UDP has no queuing semantics), which is logged under `-log-level debug`.
- **`-idle-timeout`**: under UDP a value of `0` **falls back to the default 60s**, i.e. sessions idle for more than 60s are reclaimed (under TCP, `0` means disabled).
- Direct TCP/UDP targets that resolve to the active forward listener are rejected during startup. Direct TCP checks the connected peer again after dialing so DNS changes cannot create a recursive connection or packet storm. SSH forwarding targets belong to the remote namespace and are not compared with the local listener.

## Observability

These commands replace earlier examples. Stop any instance using the same listener ports first.

In `go` mode:

- Each connection prints one log line on establishment and one on closing, including the connection sequence number, both endpoints' addresses, upload/download byte counts, and connection duration.
- Maintains an atomic counter of active connections.
- On Unix-like platforms, set `PORTMAP_PID` to the actual portmap process ID, then send `SIGUSR1` to print a connection snapshot:

```bash
kill -USR1 "$PORTMAP_PID"
# log output: status: active=<current active connections> total=<cumulative connections> rejected=<rejected> dial-errors=<dial failures> up=<upload bytes> down=<download bytes> uptime=<uptime>
```

- With `-stats-addr` you can additionally expose a read-only HTTP stats endpoint (both `forward` and `proxy`), e.g.:

```bash
portmap forward -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:8080 -stats-addr 127.0.0.1:9090
# or portmap proxy -stats-addr 127.0.0.1:9090

```

Verify from another terminal:

```bash
curl --noproxy '*' http://127.0.0.1:9090/stats     # JSON snapshot
curl --noproxy '*' http://127.0.0.1:9090/metrics   # Prometheus text
```

  The endpoint is disabled by default and, for safety, only binds to loopback addresses; under `proxy` you must pass `-stats-allow-public` to bind a non-loopback address. In multi-instance (multi-mapping) setups a single aggregated endpoint is started, summing the snapshots of all instances.

- With `-web-addr` you can enable an optional **Web panel** (both `forward` and `proxy`) to view live performance stats and access/connection logs in your browser:

```bash
portmap forward -listen-host 127.0.0.1 -listen-port 10024 -target 127.0.0.1:8080 -web-addr 127.0.0.1:9080
# or portmap proxy -web-addr 127.0.0.1:9080
```

  Then open `http://127.0.0.1:9080/` in a browser: the top of the page shows live performance cards (active/total/rejected connections, dial errors, up/down bytes, uptime), followed by a structured connection-event log table, all refreshed by automatic polling. It is a browser page rather than a `curl` endpoint, but it also exposes `/api/stats` and `/api/logs` JSON endpoints for programmatic access.

  The panel is disabled by default and, for safety (connection logs include target addresses), only binds to loopback addresses; under `proxy` you must pass `-web-allow-public` to bind a non-loopback address. Use `-web-log-max` to tune how many connection events the ring buffer keeps (default 1000).

- `-quiet` suppresses routine logs; `-log-level debug` outputs more detailed information (such as `pipe`-layer anomalies).


## Troubleshooting

### Missing flags, or `proxy` does not start

```bash
type -a portmap
portmap version
portmap forward -help
portmap proxy -help
```

Confirm that the command resolves to the intended binary. Older releases may lack subcommands or flags described here; build the current source when those features are needed.

### `address already in use`

The local listener port is occupied. Stop the process using it or change `-listen-port` / `-addr`. For SSH forwarding, changing the local port does not require changing the remote `-target`. Use `ss -lntp` on Linux, or `lsof -nP -iTCP:10024 -sTCP:LISTEN` on macOS.

### SSH connects, but the target returns `Connection refused`

The SSH transport is established, but the application target refused the connection. On the SSH server, check that the application is running, listening on the expected address and port, and has any required container ports published.

After logging into the remote server, test the target. The listener-list command below is Linux-specific:

```bash
curl --noproxy '*' -v --connect-timeout 5 http://127.0.0.1:10024/
ss -lntp 'sport = :10024'
```

### Authentication or host-key errors

Ensure `-upstream-identity` points at a private key. Check loaded agent keys with `ssh-add -l`. Non-interactive processes cannot wait for password input and need credentials configured in advance. For host-key failures, verify the host fingerprint and known_hosts entry instead of routinely disabling verification.

### No page appears after startup

Forward needs a running target service; proxy needs the client to use its proxy listener. Use the earlier curl examples to distinguish these paths, and check browser bypass rules for local addresses. Default idle timeouts can close long-idle connections; adjust `-idle-timeout` for the protocol when needed.

## Development and maintenance

- [Contributing](CONTRIBUTING.md#contributing-english): environment, checks, coding guidelines, and release workflow.
- [Performance testing](docs/performance.en.md): TCP/UDP echo workloads, sampling, and output options.
- [CHANGELOG](CHANGELOG.md): version history and upgrade impact.
- [CI workflow](.github/workflows/ci.yml) and [release workflow](.github/workflows/release.yml): configured checks and release targets.

## Project Structure

- `main.go`, `config.go`: command dispatch, option merging, and multi-instance startup.
- `internal/forward`: fixed-target TCP/UDP forwarding.
- `internal/proxy`: SOCKS5/HTTP handling and connection lifecycle.
- `internal/upstream`: direct, SOCKS5, HTTP CONNECT, and SSH transports.
- `internal/netutil`: copying, half-close, and idle timeouts.
- `internal/stats`, `internal/web`: counters and read-only monitoring.
- `internal/socat`, `internal/i18n`: socat execution and localized messages.
- `cmd/loadtest`: echo load-test tool.

## License

[Apache License 2.0](LICENSE), Copyright (c) 2026 soulteary.
