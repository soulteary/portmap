# Performance testing

The repository ships a standalone, self-contained load-test tool at [`cmd/loadtest`](../cmd/loadtest/main.go) to evaluate the **reliability** and **throughput** of portmap forwarding, and to report the current **host environment**.

In the default self-contained mode the tool starts an in-process echo target, brings up a forwarder via `forward.New(...)`, and then has the load client push traffic at the forwarding port, forming a full `client -> portmap -> echo` chain that runs with zero setup. Use `-external <addr>` to instead stress an already-running portmap (you must provide your own target service in that case).

External mode requires a target that echoes received bytes for the selected protocol. An ordinary HTTP service does not implement this workload. Run these `go run` commands from the repository root.

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

# CI: emit JSON and fail above 0.1% errors or a 5ms p95
go run ./cmd/loadtest -format json -max-error-rate 0.1 -max-p95 5ms
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
  -idle-timeout duration    built-in forwarder idle timeout; 0 disables TCP timeout, UDP uses 60s
  -warmup duration          warmup period; data during warmup is excluded from stats (default 1s)
  -format text|json         output format (default text)
  -max-samples int          maximum retained RTT samples using reservoir sampling (default 100000)
  -max-error-rate float     maximum error-rate percentage; exit 1 when exceeded
  -max-p95 duration         maximum p95 RTT; 0 disables the threshold
```

`-requests` is an early completion condition based on successful requests; it does
not disable `-duration`. The run therefore ends at the duration deadline even when
the target continuously fails to dial, times out, or returns errors. Repeated
failures also use a short backoff to avoid a busy loop against an unreachable target.

The report has three blocks: **Host / Runtime** (GOOS/GOARCH, CPU count, Go version, hostname, memory allocation), **Config**, and **Results** (throughput in MB/s and Gbps, req/s, conns/s in connrate mode, latency min/p50/p95/p99/max, error rate with per-category counts, and a reliability check that `ActiveConns()` returned to zero after the run). RTTs use bounded reservoir sampling so memory does not grow with request count; `-format json` provides machine-readable output.

Example output format (illustrative values):

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

[Back to the usage guide](../README.en.md)
