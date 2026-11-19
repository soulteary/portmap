# Central statistics

`portmap`, `portmap forward`, and `portmap proxy` count successfully forwarded
bytes locally while connections remain open. An optional reporter sends one batch
per process every second over a persistent connection. The collector never polls
forwarders. Existing `-stats-addr` and `-web-addr` endpoints remain available.

## Unix socket (same machine)

Create a private directory and start the collector:

```sh
mkdir -p .portmap-stats
chmod 700 .portmap-stats
endpoint="unix://$(pwd)/.portmap-stats/stats.sock"
./portmap stats -listen "$endpoint" \
  -journal ./.portmap-stats/history.journal -http-addr 127.0.0.1:9080
```

In the forwarder shell, set `endpoint` to the same absolute URL, then use any of:

```sh
./portmap forward -listen-host 127.0.0.1 -listen-port 31008 \
  -target 127.0.0.1:8080 -stats-endpoint "$endpoint"
./portmap proxy -config ./config.yaml -stats-endpoint "$endpoint"
sudo ./portmap -config ./config.yaml -stats-endpoint "$endpoint"
```

These are separate example commands. SSH upstream forwarding uses the same
reporter. `socat` mode does not support reporting and rejects this option.

Without `-listen`, the collector creates a private per-user directory under the OS
temporary directory and prints its socket address at startup. Reporters always
require an explicit endpoint. A Unix socket is mode `0600`; use the same account
(or root) to connect. Custom socket parent directories must already exist. An
existing socket is never removed automatically on startup. After a crash, verify
that no collector is listening before removing a stale socket file.

Configuration uses `stats_endpoint` inside `forward` or `proxy` (also supported in
legacy flat forward configurations):

```yaml
forward:
  listen_port: 31008
  target: 127.0.0.1:8080
  stats_endpoint: unix:///absolute/private/directory/stats.sock
proxy:
  addr: 127.0.0.1:8118
  stats_endpoint: unix:///absolute/private/directory/stats.sock
```

For multi-instance configurations, reporting is process-wide. Nonempty endpoints
must agree; one batch includes all instances. The explicit CLI flag overrides
instance configuration, including `-stats-endpoint=''` to disable reporting.
Instance IDs are list positions within a process lifetime; labels include the
listener and, for forwarding, the target. A new process boot ID separates restarts.

## TCP

```sh
./portmap stats -listen tcp://127.0.0.1:9190 \
  -journal ./history.journal -http-addr 127.0.0.1:9080
./portmap proxy -config ./config.yaml -stats-endpoint tcp://127.0.0.1:9190
```

The collector requires a loopback IP by default. `-allow-public` explicitly permits
other TCP bind addresses for a trusted network or separately secured tunnel. The
transport has protocol-version negotiation but no authentication or encryption;
port reachability is not proof of identity. The HTTP query listener always requires
a loopback IP. `-http-addr=''` disables it.

## Queries and storage

Open `http://127.0.0.1:9080/` for the existing monitoring panel. The same listener
provides:

- `/stats` and `/api/stats`: aggregate counters.
- `/metrics`: aggregate Prometheus metrics.
- `/api/logs`: the most recent 1000 connection events.
- `/api/sources`: the last batch for each process, with receiver timestamps.
- `/api/history`: the most recent 1000 accepted batches, excluding event payloads.

A source older than three seconds is excluded from the aggregate active-connection
gauge; its cumulative counters remain included. Source records retain their last
reported gauges, so consumers must check `received_at` before treating them as live.

The journal stores length-prefixed JSON records. It is append-only, locked against
concurrent collectors, and synced before each acknowledgement. Restart replays the
journal to restore sequence numbers, cumulative totals, recent history, and events.
An incomplete final frame is discarded; malformed frames and invalid snapshots stop startup. Duplicate
or older sequences do not append records. Decreasing cumulative counters within
one process boot are rejected. The collector sums the latest cumulative snapshot
per process rather than summing every batch.

This version does not compact or rotate the journal. Plan disk retention and stop
the collector before archiving or replacing it. Starting with an empty journal
resets collector history. At most 10,000 process lifetimes are retained per journal;
each batch supports up to 1024 instances. Receiver concurrency and frame size are
also bounded (128 connections, 4 MiB per frame).

## Failure and accounting semantics

- Forwarding does not wait for collection. Missing, slow, incompatible, or
  disconnected collectors cause background retries with exponential backoff and
  jitter (up to approximately 38 seconds between attempts, plus dial and handshake time).
- Only the latest cumulative snapshot is needed after an outage. A later snapshot
  fills the total without reconstructing the per-second distribution of bytes.
- Events share the bounded local event log (`web_log_max`, default 1000). Each
  report includes at most 1000 pending events; overflow is reflected in
  `events_dropped`. Delivery is best effort. After an unacknowledged batch, this
  count can conservatively include events already persisted by the collector.
- Forwarders do not persist locally or wait for a final report during shutdown.
  Bytes after the last accepted sample can be lost if the process exits. Under
  normal operation the reporting window is about one second; outages extend it.
- Event client and target fields and instance labels are limited to 256 bytes on
  the reporting path. Local logs retain their existing values. Collector events
  include a process `source` ID because connection IDs are only unique per process.
- Byte counters describe successful forwarding writes, including partial writes.
  They exclude SSH framing/encryption and TCP/IP overhead. Proxy negotiation replies
  are outside tunnel payload accounting. Plain HTTP counts its rewritten messages.
- Real-time counting uses pooled 32 KiB userspace copy buffers. This disables the
  raw TCP `io.Copy`/Linux splice fast path for counted relays, including when idle
  timeouts are disabled. No throughput improvement is claimed by this change.
