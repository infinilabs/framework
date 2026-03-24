---
title: "Key-Value Store"
weight: 35
---
# Key-Value Store

The INFINI Framework provides a pluggable key-value store abstraction for persisting arbitrary binary data organized by buckets. The KV store decouples application logic from the underlying storage engine, allowing you to swap backends — BadgerDB, Elasticsearch, or a simple file-based store — without changing your application code. All backends are accessed through a unified API with built-in compression support.

## KVStore Interface

Every KV backend must implement the `KVStore` interface defined in `core/kv/`. This interface provides lifecycle management and CRUD operations on bucket-scoped key-value pairs.

```go
type KVStore interface {
    Open() error
    Close() error
    GetValue(bucket string, key []byte) ([]byte, error)
    GetCompressedValue(bucket string, key []byte) ([]byte, error)
    AddValueCompress(bucket string, key []byte, value []byte) error
    AddValue(bucket string, key []byte, value []byte) error
    ExistsKey(bucket string, key []byte) (bool, error)
    DeleteKey(bucket string, key []byte) error
}
```

| Method | Description |
|--------|-------------|
| `Open() error` | Opens the underlying storage engine and prepares it for reads and writes. Called during module startup. |
| `Close() error` | Closes the storage engine, flushing any pending data to disk. Called during shutdown. |
| `GetValue(bucket, key)` | Retrieves the raw byte value associated with `key` in the given `bucket`. Returns `nil` if the key does not exist. |
| `GetCompressedValue(bucket, key)` | Retrieves a value that was stored with LZ4 compression and returns the decompressed bytes. |
| `AddValueCompress(bucket, key, value)` | Compresses `value` using LZ4 and stores the result under `key` in the given `bucket`. |
| `AddValue(bucket, key, value)` | Stores a raw byte `value` under `key` in the given `bucket`. Overwrites any existing value. |
| `ExistsKey(bucket, key)` | Checks whether `key` exists in the given `bucket` without retrieving the value. |
| `DeleteKey(bucket, key)` | Removes `key` and its associated value from the given `bucket`. |

## Package-Level Functions

The `kv` package exposes convenience functions that delegate to the currently registered backend. These allow you to perform KV operations without holding a direct reference to the store instance.

```go
import "infini.sh/framework/core/kv"

// Store and retrieve values
kv.AddValue(bucket string, key []byte, value []byte) error
kv.GetValue(bucket string, key []byte) ([]byte, error)

// Store and retrieve compressed values
kv.AddValueCompress(bucket string, key []byte, value []byte) error
kv.GetCompressedValue(bucket string, key []byte) ([]byte, error)

// Check existence and delete
kv.ExistsKey(bucket string, key []byte) (bool, error)
kv.DeleteKey(bucket string, key []byte) error
```

These functions panic if no backend has been registered. Ensure that a KV backend module is imported before calling them.

## Registering Backends

KV backends register themselves using `kv.Register`. Registration typically happens inside a module's `Setup()` method, which is called during application initialization.

```go
kv.Register(name string, h KVStore)
```

| Parameter | Description |
|-----------|-------------|
| `name` | A unique name identifying the backend (e.g., `"badger"`, `"elastic"`, `"simple_kv"`). |
| `h` | An implementation of the `KVStore` interface. |

Registering a backend with the same name as an existing one causes a panic. The most recently registered backend becomes the active handler for all package-level functions.

### Registration Pattern

```go
package my_kv

import (
    "infini.sh/framework/core/kv"
    "infini.sh/framework/core/module"
)

type MyKVBackend struct{}

func (m *MyKVBackend) Setup() {
    kv.Register("my_backend", m)
}

func init() {
    module.RegisterModuleWithPriority(&MyKVBackend{}, -100)
}
```

## CRUD Operations

### Storing a Value

Use `AddValue` to store a raw byte value under a bucket and key:

```go
bucket := "user_sessions"
key := []byte("session-abc-123")
value := []byte(`{"user_id": "u1", "expires": "2024-12-31"}`)

err := kv.AddValue(bucket, key, value)
if err != nil {
    log.Error("failed to store session:", err)
}
```

### Retrieving a Value

Use `GetValue` to read a value back:

```go
data, err := kv.GetValue("user_sessions", []byte("session-abc-123"))
if err != nil {
    log.Error("failed to get session:", err)
    return
}

if data == nil {
    fmt.Println("session not found")
    return
}

fmt.Printf("session data: %s\n", string(data))
```

### Checking Key Existence

Use `ExistsKey` to check whether a key exists without retrieving its value:

```go
exists, err := kv.ExistsKey("user_sessions", []byte("session-abc-123"))
if err != nil {
    log.Error("existence check failed:", err)
    return
}

if exists {
    fmt.Println("session exists")
}
```

### Deleting a Key

Use `DeleteKey` to remove a key and its associated value:

```go
err := kv.DeleteKey("user_sessions", []byte("session-abc-123"))
if err != nil {
    log.Error("failed to delete session:", err)
}
```

## Compression Support

The KV store provides built-in LZ4 compression for values that benefit from reduced storage size. Compressed and uncompressed values use separate method pairs — you must retrieve a value using the same mode it was stored with.

### Storing Compressed Values

```go
bucket := "documents"
key := []byte("doc-001")
largeValue := []byte(`{"title": "...", "content": "very large document body..."}`)

// Store with LZ4 compression
err := kv.AddValueCompress(bucket, key, largeValue)
if err != nil {
    log.Error("failed to store compressed value:", err)
}
```

### Retrieving Compressed Values

```go
// Retrieve and automatically decompress
data, err := kv.GetCompressedValue("documents", []byte("doc-001"))
if err != nil {
    log.Error("failed to get compressed value:", err)
    return
}

fmt.Printf("document: %s\n", string(data))
```

> **Note:** Calling `GetValue` on a compressed entry returns the raw LZ4-encoded bytes. Always use `GetCompressedValue` to retrieve values that were stored with `AddValueCompress`.

## Bucket Organization

Buckets provide logical namespacing for keys. Each bucket acts as an independent keyspace, so the same key can exist in multiple buckets without conflict.

```go
// Store the same key in different buckets
kv.AddValue("cache",    []byte("config"), []byte(`{"ttl": 60}`))
kv.AddValue("settings", []byte("config"), []byte(`{"theme": "dark"}`))

// Retrieve from specific buckets
cacheVal, _ := kv.GetValue("cache",    []byte("config"))  // {"ttl": 60}
settingsVal, _ := kv.GetValue("settings", []byte("config"))  // {"theme": "dark"}
```

Common bucket naming conventions:

| Bucket Name | Purpose |
|-------------|---------|
| `"cache"` | Temporary cached data |
| `"state"` | Persistent component state |
| `"metadata"` | Object metadata and indexes |
| `"sessions"` | User or API session data |

In the Badger backend with `single_bucket_mode: true` (the default), bucket names are prefixed onto keys internally, so all data resides in a single underlying database while maintaining logical separation.

## Available Backends

The framework ships with three KV backend implementations. Each backend is activated by importing its module package.

### Badger (Default)

BadgerDB is a high-performance embedded key-value store written in Go. It is the default and recommended backend for most use cases, offering fast reads and writes with optional compression and garbage collection.

**Activation:**

```go
import _ "infini.sh/framework/plugins/badger"
```

**Configuration:**

```yaml
badger:
  enabled: true
  single_bucket_mode: true
  path: "data/badger"
  memory_mode: false
  sync_writes: false
  mem_table_size: 67108864
  value_log_file_size: 1073741824
  value_threshold: 1048576
  value_log_max_entries: 1000000
  num_mem_tables: 1
  num_level0_tables: 1
  num_level0_tables_stall: 2
  value_log_gc_enabled: true
  value_log_gc_discard_ratio: 0.5
  value_log_gc_interval_in_seconds: 120
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | `bool` | `true` | Enable or disable the Badger backend. |
| `single_bucket_mode` | `bool` | `true` | When enabled, all buckets share a single underlying database with bucket names prefixed to keys. |
| `path` | `string` | `data/badger` | Directory path for database files. Defaults to `<data_dir>/badger`. |
| `memory_mode` | `bool` | `false` | Run entirely in memory. Data is lost on restart. |
| `sync_writes` | `bool` | `false` | Force sync to disk on every write. Increases durability at the cost of throughput. |
| `mem_table_size` | `int64` | `10485760` | Size of each in-memory table in bytes. |
| `value_log_file_size` | `int64` | `1073741823` | Maximum size of each value log file in bytes. |
| `value_threshold` | `int64` | `1048576` | Values larger than this threshold are stored in the value log. |
| `value_log_max_entries` | `uint32` | `1000000` | Maximum number of entries per value log file. |
| `num_mem_tables` | `int` | `1` | Number of memtables to keep in memory. |
| `num_level0_tables` | `int` | `1` | Number of Level 0 tables before compaction triggers. |
| `num_level0_tables_stall` | `int` | `2` | Number of Level 0 tables that triggers a write stall. |
| `value_log_gc_enabled` | `bool` | `true` | Enable automatic garbage collection of the value log. |
| `value_log_gc_discard_ratio` | `float64` | `0.5` | Minimum ratio of discarded entries before a value log file is eligible for GC. |
| `value_log_gc_interval_in_seconds` | `int` | `120` | Interval in seconds between value log GC runs. |

### Elasticsearch

The Elasticsearch backend stores key-value pairs as documents in an Elasticsearch index. Use this when you need KV data to be searchable, replicated, or integrated with an existing Elasticsearch cluster.

**Activation:**

The Elasticsearch KV backend is activated as part of the `elastic` module when the store configuration is enabled.

```go
import _ "infini.sh/framework/modules/elastic"
```

Values are Base64-encoded and stored in a `content` field. Keys are hashed using MD5 to produce deterministic document IDs.

### Simple KV

The simple KV backend is a lightweight file-based store using a write-ahead log (WAL) for durability. It periodically snapshots state to disk and replays the WAL on startup for crash recovery.

**Activation:**

```go
import _ "infini.sh/framework/plugins/simple_kv"
```

**Configuration:**

```yaml
simple_kv:
  enabled: true
  path: "data/simple_kv"
  sync_writes: false
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | `bool` | `true` | Enable or disable the simple KV backend. |
| `path` | `string` | `data/simple_kv` | Directory for state snapshots and WAL files. Defaults to `<data_dir>/simple_kv`. |
| `sync_writes` | `bool` | `false` | Force sync to disk on every write. |

## Complete Example

Below is a complete example demonstrating KV store initialization, CRUD operations, and compression using the Badger backend.

```go
package main

import (
    "fmt"

    log "github.com/cihub/seelog"
    "infini.sh/framework/core/kv"
    _ "infini.sh/framework/plugins/badger"
)

func main() {
    // The Badger module registers itself on import.
    // After framework initialization, the KV store is ready to use.

    bucket := "example"

    // --- Create ---
    err := kv.AddValue(bucket, []byte("greeting"), []byte("hello world"))
    if err != nil {
        log.Errorf("add failed: %v", err)
        return
    }
    fmt.Println("stored: greeting -> hello world")

    // --- Read ---
    val, err := kv.GetValue(bucket, []byte("greeting"))
    if err != nil {
        log.Errorf("get failed: %v", err)
        return
    }
    fmt.Printf("retrieved: greeting -> %s\n", string(val))

    // --- Exists ---
    exists, err := kv.ExistsKey(bucket, []byte("greeting"))
    if err != nil {
        log.Errorf("exists check failed: %v", err)
        return
    }
    fmt.Printf("exists: %v\n", exists)

    // --- Compressed Write/Read ---
    largeData := []byte(`{"payload": "large document content that benefits from compression..."}`)

    err = kv.AddValueCompress(bucket, []byte("doc-001"), largeData)
    if err != nil {
        log.Errorf("compressed add failed: %v", err)
        return
    }
    fmt.Println("stored compressed: doc-001")

    decompressed, err := kv.GetCompressedValue(bucket, []byte("doc-001"))
    if err != nil {
        log.Errorf("compressed get failed: %v", err)
        return
    }
    fmt.Printf("retrieved compressed: doc-001 -> %s\n", string(decompressed))

    // --- Delete ---
    err = kv.DeleteKey(bucket, []byte("greeting"))
    if err != nil {
        log.Errorf("delete failed: %v", err)
        return
    }

    // Verify deletion
    val, err = kv.GetValue(bucket, []byte("greeting"))
    if err != nil {
        log.Errorf("get after delete failed: %v", err)
        return
    }
    if val == nil {
        fmt.Println("deleted: greeting (confirmed nil)")
    }
}
```

### YAML Configuration

```yaml
badger:
  enabled: true
  single_bucket_mode: true
  path: "data/badger"
  memory_mode: false
  sync_writes: false
  mem_table_size: 67108864
  value_log_file_size: 1073741824
  value_log_gc_enabled: true
  value_log_gc_discard_ratio: 0.5
  value_log_gc_interval_in_seconds: 120
```
