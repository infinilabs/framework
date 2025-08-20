---
title: "Simple KV Plugin"
weight: 30
---

# Simple KV Plugin

The Simple KV plugin provides a lightweight, file-based key-value storage solution with write-ahead logging (WAL) support. It's designed for scenarios where you need persistent storage without the complexity of a full database system.

## Features

- File-based key-value storage
- Write-ahead logging (WAL) for data durability
- Simple and lightweight implementation
- Filter and KV storage interface support
- Configurable synchronous/asynchronous writes
- Automatic data directory management

## Configuration

Configure the Simple KV plugin in your YAML configuration:

```yaml
simple_kv:
  enabled: true
  path: "./data/simple_kv"
  sync_writes: false
```

## Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable/disable the Simple KV plugin |
| `path` | string | `"./data/simple_kv"` | Directory path for data storage |
| `sync_writes` | boolean | `false` | Enable synchronous writes to disk |

## Storage Structure

The Simple KV plugin creates the following directory structure:

```
simple_kv/
├── last_state/     # Current key-value state files
└── wal/           # Write-ahead log files
```

### Write-Ahead Logging

The plugin uses WAL to ensure data durability:
- All write operations are first logged to the WAL
- State files are updated after WAL write completion
- Recovery is performed automatically on startup
- WAL files are rotated to prevent unlimited growth

## Usage as Key-Value Store

The plugin registers itself with the framework's KV system:

```go
import "infini.sh/framework/core/kv"

// Access the Simple KV store
store := kv.GetStore("simple_kv")

// Store a value
err := store.Set("my_key", []byte("my_value"))
if err != nil {
    log.Error("Failed to store value:", err)
}

// Retrieve a value
value, err := store.Get("my_key")
if err != nil {
    log.Error("Failed to retrieve value:", err)
}

// Delete a value
err = store.Delete("my_key")
if err != nil {
    log.Error("Failed to delete value:", err)
}
```

## Usage as Filter

The plugin also registers as a filter for membership testing:

```go
import "infini.sh/framework/core/filter"

// Access the Simple KV filter
kvFilter := filter.GetFilter("simple_kv")

// Add an item to the filter
err := kvFilter.Add([]byte("item_to_track"))
if err != nil {
    log.Error("Failed to add item:", err)
}

// Check if an item exists
exists := kvFilter.Contains([]byte("item_to_track"))
if exists {
    log.Info("Item found in filter")
}
```

## Performance Characteristics

### Write Performance
- **Asynchronous writes** (`sync_writes: false`): Higher throughput, potential data loss on crash
- **Synchronous writes** (`sync_writes: true`): Lower throughput, guaranteed durability

### Read Performance
- In-memory caching for frequently accessed keys
- Sequential file access patterns for optimal disk I/O
- Efficient key lookup using internal indexing

### Storage Efficiency
- Compact binary storage format
- Automatic compression of stored values
- WAL cleanup and compaction

## Data Durability

### Write-Ahead Logging
1. **Write to WAL**: All changes are first written to the WAL
2. **Apply to state**: Changes are applied to the main state files
3. **WAL cleanup**: Successfully applied entries are removed from WAL

### Recovery Process
1. **Startup scan**: Plugin scans WAL files on startup
2. **Replay operations**: Unapplied WAL entries are replayed
3. **State reconstruction**: Current state is reconstructed from WAL
4. **Normal operation**: Plugin continues with normal operations

## Configuration Examples

### High-Performance Setup
```yaml
simple_kv:
  enabled: true
  path: "/fast-ssd/simple_kv"
  sync_writes: false  # Higher throughput
```

### High-Durability Setup
```yaml
simple_kv:
  enabled: true
  path: "./data/simple_kv"
  sync_writes: true   # Guaranteed durability
```

### Development/Testing Setup
```yaml
simple_kv:
  enabled: true
  path: "/tmp/simple_kv"
  sync_writes: false
```

## Integration

The Simple KV plugin integrates with:

- **Core KV system** - Provides key-value storage interface
- **Filter system** - Offers membership testing capabilities
- **Global environment** - Uses data directory configuration
- **Logging system** - Provides operational logging

## Monitoring

Monitor Simple KV performance through:
- Framework stats system
- File system monitoring
- Application logs for error tracking

## Use Cases

### Configuration Storage
Store application configuration that needs to persist:
```go
// Store configuration
kv.Set("app_config", configData)

// Retrieve on startup
configData, _ := kv.Get("app_config")
```

### Session Management
Maintain user session data:
```go
// Store session
sessionData := []byte(`{"user_id": 123, "expires": "2024-01-01"}`)
kv.Set("session_"+sessionID, sessionData)

// Check session exists
exists := filter.Contains([]byte("session_"+sessionID))
```

### Cache Implementation
Simple caching for computed values:
```go
// Cache computed result
result := computeExpensiveOperation()
kv.Set("cache_"+key, result)

// Retrieve from cache
cached, err := kv.Get("cache_"+key)
```

## Best Practices

1. **Path Configuration**: Use fast storage (SSD) for better performance
2. **Sync Writes**: Enable for critical data, disable for high-throughput scenarios
3. **Key Design**: Use consistent key naming patterns
4. **Data Size**: Keep individual values reasonably small for best performance
5. **Monitoring**: Monitor disk space usage in the data directory

## Troubleshooting

### Common Issues

1. **Plugin not starting**
   - Check directory permissions on the data path
   - Verify available disk space
   - Review startup logs for errors

2. **Performance issues**
   - Consider disabling `sync_writes` for higher throughput
   - Move data directory to faster storage
   - Monitor disk I/O patterns

3. **Data corruption**
   - Simple KV includes WAL-based recovery
   - Check file system health
   - Review error logs for write failures

### Debug Information

Monitor Simple KV operations:
- Check plugin registration in startup logs
- Monitor file creation in the data directory
- Review WAL file growth and cleanup patterns

### Recovery Procedures

If data appears corrupted:
1. Stop the application
2. Check WAL files for recent operations
3. The plugin will automatically recover on next startup
4. Monitor logs for recovery progress