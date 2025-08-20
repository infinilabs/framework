---
title: "Badger Plugin"
weight: 10
---

# Badger Plugin

The Badger plugin provides a high-performance key-value storage backend using BadgerDB. It offers embedded persistent storage with configurable memory management, garbage collection, and performance tuning options.

## Features

- High-performance embedded key-value storage
- Configurable memory management and caching
- Automatic garbage collection for value logs
- Single bucket mode for simplified operations
- Optional in-memory mode for testing
- Statistics API endpoint

## Configuration

Configure the Badger plugin in your YAML configuration:

```yaml
badger:
  enabled: true
  single_bucket_mode: true
  path: "./data/badger"
  memory_mode: false
  sync_writes: false
  
  # Memory Configuration
  mem_table_size: 10485760  # 10MB
  num_mem_tables: 1
  
  # Value Log Configuration  
  value_log_file_size: 1073741823  # ~1GB
  value_threshold: 1048576         # 1MB
  value_log_max_entries: 1000000   # 1M entries
  
  # Level 0 Tables
  num_level0_tables: 1
  num_level0_tables_stall: 2
  
  # Garbage Collection
  value_log_gc_enabled: true
  value_log_gc_discard_ratio: 0.5
  value_log_gc_interval_in_seconds: 120
```

## Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable/disable the Badger plugin |
| `single_bucket_mode` | boolean | `true` | Use single bucket for all operations |
| `path` | string | `"./data/badger"` | Directory path for data storage |
| `memory_mode` | boolean | `false` | Enable in-memory only mode |
| `sync_writes` | boolean | `false` | Synchronous writes to disk |
| `mem_table_size` | int64 | `10485760` | Size of each memtable in bytes |
| `num_mem_tables` | int | `1` | Number of memtables to maintain |
| `value_log_file_size` | int64 | `1073741823` | Maximum size of value log files |
| `value_threshold` | int64 | `1048576` | Threshold for storing values in value log |
| `value_log_max_entries` | uint32 | `1000000` | Maximum entries in value log |
| `num_level0_tables` | int | `1` | Number of Level 0 tables |
| `num_level0_tables_stall` | int | `2` | Stall writes when this many L0 tables |
| `value_log_gc_enabled` | boolean | `true` | Enable value log garbage collection |
| `value_log_gc_discard_ratio` | float64 | `0.5` | Discard ratio for garbage collection |
| `value_log_gc_interval_in_seconds` | int | `120` | GC interval in seconds |

## API Endpoints

The Badger plugin provides the following API endpoint:

- **GET `/badger/stats`** - Returns storage statistics including:
  - Database size information
  - Memory usage statistics  
  - Garbage collection metrics
  - Performance counters

Example response:
```json
{
  "lsm_size": 1048576,
  "vlog_size": 2097152,
  "pending_writes": 0,
  "num_reads": 12345,
  "num_writes": 6789
}
```

## Usage

The Badger plugin automatically registers itself as both a filter and key-value storage provider:

### As Key-Value Storage
```go
// The plugin registers with the kv package
import "infini.sh/framework/core/kv"

// Access through the KV interface
store := kv.GetStore("badger")
```

### As Filter
```go
// The plugin also registers as a filter
import "infini.sh/framework/core/filter"

// Access through the filter interface  
filter := filter.GetFilter("badger")
```

## Performance Tuning

### Memory Usage
- Increase `mem_table_size` for better write performance
- Adjust `num_mem_tables` based on available memory
- Use `memory_mode` for testing or temporary storage

### Write Performance
- Disable `sync_writes` for better throughput
- Increase `value_threshold` to keep more data in LSM tree
- Tune `num_level0_tables` for write stall behavior

### Garbage Collection
- Adjust `value_log_gc_discard_ratio` based on update patterns
- Modify `value_log_gc_interval_in_seconds` for GC frequency
- Monitor via `/badger/stats` endpoint

## Integration

The Badger plugin integrates with:

- **Core KV system** - Provides key-value storage interface
- **Filter system** - Offers filtering capabilities
- **API module** - Exposes statistics endpoint
- **Global environment** - Uses data directory configuration

## Best Practices

1. **Production Configuration**: Enable `sync_writes` for data durability
2. **Memory Management**: Monitor memory usage and adjust memtable settings
3. **Storage Planning**: Plan for value log file growth and cleanup
4. **Monitoring**: Use the stats API to monitor performance
5. **Backup**: Ensure proper backup of the data directory

## Troubleshooting

- **High Memory Usage**: Reduce `mem_table_size` or `num_mem_tables`
- **Slow Writes**: Check `num_level0_tables_stall` setting
- **Large Storage**: Verify garbage collection is running effectively
- **Startup Issues**: Check directory permissions and available disk space