---
title: "Statistics Collection"
weight: 40
---

# Statistics Collection

The INFINI Framework provides a pluggable statistics collection system for recording, querying, and exporting application metrics. The `core/stats` package defines a common interface and package-level convenience functions, while backends such as the built-in `SimpleStatsModule` and the optional StatsD plugin handle storage and forwarding.

## Architecture

The stats system follows a handler-based design:

1. **Core interface** (`core/stats`) — defines the `StatsInterface` contract and exposes package-level functions that delegate to every registered backend.
2. **Built-in module** (`modules/stats`) — `SimpleStatsModule` stores counters in memory with optional persistence to disk, and serves them over HTTP in JSON and Prometheus formats.
3. **StatsD plugin** (`plugins/stats_statsd`) — forwards every metric operation to an external StatsD server over UDP or TCP.

Multiple backends can be registered at the same time; every write operation fans out to all of them.

## StatsInterface

Any backend must implement the `StatsInterface`:

```go
type StatsInterface interface {
    Increment(category, key string)
    IncrementBy(category, key string, value int64)
    Decrement(category, key string)
    DecrementBy(category, key string, value int64)
    Absolute(category, key string, value int64)
    Timing(category, key string, v int64)
    Gauge(category, key string, v int64)
    Stat(category, key string) int64
    StatsAll() string
    RecordTimestamp(category, key string, value time.Time)
    GetTimestamp(category, key string) (time.Time, error)
}
```

Register a backend with `stats.Register`:

```go
stats.Register(myBackend)
```

## Recording Metrics

All package-level functions are safe to call even when no backend is registered — they silently return.

### Increment / Decrement

Counters are the most common metric type. `Increment` accepts a variadic `key` that is joined with `"."`:

```go
// Increment by 1 — variadic keys are joined with "."
stats.Increment("pipeline", "requests", "total")   // category="pipeline", key="requests.total"

// Increment by an arbitrary value
stats.IncrementBy("pipeline", "bytes_received", int64(n))

// Decrement by 1
stats.Decrement("queue", "pending")

// Decrement by an arbitrary value
stats.DecrementBy("queue", "pending", int64(batchSize))
```

### Absolute

Set a counter to an exact value, replacing whatever was stored before:

```go
stats.Absolute("cluster", "active_nodes", 5)
```

### Timing

Record a timing value (e.g. latency in milliseconds):

```go
start := time.Now()
// ... do work ...
stats.Timing("pipeline", "process_latency_ms", time.Since(start).Milliseconds())
```

### Gauge

Set a point-in-time measurement that can go up or down:

```go
stats.Gauge("queue", "depth", int64(currentDepth))
```

> **Note:** In the built-in `SimpleStatsModule`, both `Absolute` and `Gauge` set the value directly. The StatsD plugin forwards them as the corresponding StatsD metric types.

## Reading Metrics

### Stat

Retrieve a single counter value:

```go
total := stats.Stat("pipeline", "requests.total")
```

### StatsMap

`StatsMap` returns all metrics — including counters, system information, and any registered computed stats — as a `util.MapStr`:

```go
metrics, err := stats.StatsMap()
if err != nil {
    log.Error(err)
    return
}
// metrics is a map containing "stats", "system", "pool", "disk", and any registered keys
```

The returned map has the following top-level structure:

| Key      | Description |
|----------|-------------|
| `stats`  | All recorded counters organised by category and key. |
| `system` | Runtime metrics: `uptime_in_ms`, `cpu`, `mem`, `goroutines`, `objects`, `stack`, `mspan`, `gc`, `cgo_calls`, `user_in_ms`, `sys_in_ms`, and optionally `store`. |
| `pool`   | Byte-buffer pool statistics. |
| `disk`   | Disk partition usage for the data directory (when `include_storage_stats_in_api` is enabled). |

## Timestamps

The stats package can record the last time a particular operation occurred. This is useful for tracking "last seen" or "last processed" events.

```go
// Record the current time
stats.TimestampNow("crawler", "last_run")

// Record a specific time
stats.Timestamp("crawler", "last_run", specificTime)

// Retrieve a recorded timestamp (returns nil when not found)
ts := stats.GetTimestamp("crawler", "last_run")
if ts != nil {
    fmt.Println("Last run at:", *ts)
}
```

`TimestampNow` and `GetTimestamp` accept variadic keys that are joined with `"."`, matching the behaviour of `Increment`.

## Registering Custom Stats

Use `RegisterStats` to contribute computed metrics that are merged into the output of `StatsMap` and the `/stats` API endpoint:

```go
stats.RegisterStats("my_component", func() interface{} {
    return map[string]interface{}{
        "cache_hit_ratio": computeHitRatio(),
        "open_connections": pool.ActiveCount(),
    }
})
```

The callback is invoked every time `StatsMap()` is called. The returned value is stored under the key you provide (`"my_component"` in the example above).

## Built-in Stats Module

The `SimpleStatsModule` (`modules/stats`) is enabled by default and provides in-process metric storage, persistence, and HTTP endpoints.

### Configuration

Add a `stats` section to your application configuration file:

```yaml
stats:
  enabled: true
  persist: true
  no_buffer: true
  buffer_size: 1000
  flush_interval_ms: 1000
  include_storage_stats_in_api: true
```

| Parameter                     | Type   | Default | Description |
|-------------------------------|--------|---------|-------------|
| `enabled`                     | `bool` | `true`  | Enable or disable the stats module. |
| `persist`                     | `bool` | `true`  | Persist counters to disk on shutdown and reload on startup. Data is stored under `{data_dir}/stats/`. |
| `no_buffer`                   | `bool` | `true`  | When `true`, counter writes go directly to the in-memory map (synchronous mode). When `false`, writes are queued in a lock-free ring buffer and flushed periodically. |
| `buffer_size`                 | `int`  | `1000`  | Size of the lock-free queue when `no_buffer` is `false`. |
| `flush_interval_ms`           | `int`  | `1000`  | Flush interval in milliseconds for the buffered mode. Minimum enforced value is `100`. |
| `include_storage_stats_in_api`| `bool` | `true`  | Include disk usage and data-directory size in the `/stats` API response. |

### API Endpoints

The module registers the following HTTP endpoints on the API server:

| Method   | Path                          | Description |
|----------|-------------------------------|-------------|
| `GET`    | `/stats`                      | Returns all metrics as JSON. Append `?format=prometheus` to get Prometheus text format instead. |
| `GET`    | `/stats/prometheus`           | Returns all metrics in Prometheus text exposition format. |
| `GET`    | `/debug/goroutines`           | Returns goroutine stacks grouped by call site with occurrence counts. |
| `GET`    | `/debug/pool/bytes`           | Returns byte-buffer pool item size statistics. |

#### JSON Response

A `GET /stats` request returns a JSON object:

```json
{
  "stats": {
    "pipeline": {
      "requests.total": 18420,
      "bytes_received": 5242880
    }
  },
  "system": {
    "uptime_in_ms": 360000,
    "cpu": 12,
    "mem": 134217728,
    "goroutines": 42,
    "objects": 98304,
    "stack": 1048576,
    "mspan": 65536,
    "gc": 15,
    "cgo_calls": 3,
    "user_in_ms": 5400,
    "sys_in_ms": 1200,
    "store": 52428800
  },
  "pool": { },
  "disk": { }
}
```

#### Prometheus Response

A `GET /stats/prometheus` (or `GET /stats?format=prometheus`) request returns plain-text metrics with labels:

```
stats_pipeline_requests_total{type="myapp", ip="192.168.1.10", name="node-1", id="abc123"} 18420
stats_pipeline_bytes_received{type="myapp", ip="192.168.1.10", name="node-1", id="abc123"} 5242880
system_goroutines{type="myapp", ip="192.168.1.10", name="node-1", id="abc123"} 42
```

Each metric name is flattened from the JSON hierarchy (dots and special characters are replaced with underscores), and every metric includes `type`, `ip`, `name`, and `id` labels derived from the node configuration.

## StatsD Integration

The StatsD plugin (`plugins/stats_statsd`) forwards metrics to an external StatsD-compatible server. Enable it by importing the plugin and adding configuration:

```yaml
statsd:
  enabled: true
  host: "localhost"
  port: 8125
  namespace: "myapp."
  protocol: "udp"
  interval_in_seconds: 1
  buffer_size: 100
```

| Parameter            | Type     | Default      | Description |
|----------------------|----------|--------------|-------------|
| `enabled`            | `bool`   | `false`      | Enable or disable the StatsD plugin. |
| `host`               | `string` | `"localhost"`| StatsD server hostname. |
| `port`               | `int`    | `8125`       | StatsD server port. |
| `namespace`          | `string` | `"app."`     | Prefix prepended to every metric name. |
| `protocol`           | `string` | `"udp"`      | Transport protocol: `"udp"` or `"tcp"`. |
| `interval_in_seconds`| `int`    | `1`          | How often the buffered client flushes metrics to the server. |
| `buffer_size`        | `int`    | `100`        | Maximum number of metrics held in the send buffer. |

When enabled, the StatsD plugin registers itself as a `StatsInterface` backend. All `Increment`, `Decrement`, `Timing`, `Gauge`, and `Absolute` calls are forwarded. Metric names are formed as `category.key` (e.g. `pipeline.requests.total`).

> **Note:** The StatsD plugin does not support `RecordTimestamp`/`GetTimestamp` or `Stat`/`StatsAll` — these operations are no-ops or return zero values.

## Complete Example

Below is a full example showing how to record, read, and expose custom metrics in an application built on the INFINI Framework:

```go
package main

import (
    "fmt"
    "time"

    "infini.sh/framework/core/stats"
)

// Record metrics during request processing
func handleRequest(size int64) {
    stats.Increment("api", "requests", "total")
    stats.IncrementBy("api", "bytes_in", size)
    stats.TimestampNow("api", "last_request")

    start := time.Now()
    // ... process request ...
    stats.Timing("api", "latency_ms", time.Since(start).Milliseconds())
}

// Track active connections with a gauge
func onConnect() {
    stats.Increment("connections", "active")
    stats.Gauge("connections", "total_active", int64(pool.Size()))
}

func onDisconnect() {
    stats.Decrement("connections", "active")
    stats.Gauge("connections", "total_active", int64(pool.Size()))
}

// Register computed stats that appear in /stats output
func init() {
    stats.RegisterStats("cache", func() interface{} {
        return map[string]interface{}{
            "size":      cache.Len(),
            "hit_ratio": cache.HitRatio(),
        }
    })
}

// Read a metric value programmatically
func reportMetrics() {
    total := stats.Stat("api", "requests.total")
    fmt.Printf("Total API requests: %d\n", total)

    ts := stats.GetTimestamp("api", "last_request")
    if ts != nil {
        fmt.Printf("Last request at: %v\n", *ts)
    }

    // Get the full stats map
    metrics, err := stats.StatsMap()
    if err == nil {
        fmt.Printf("All metrics: %v\n", metrics)
    }
}
```

With the built-in module enabled, these metrics are automatically available at `GET /stats` (JSON) and `GET /stats/prometheus` (Prometheus format). If the StatsD plugin is also enabled, the same metrics are forwarded to your StatsD server in real time.
