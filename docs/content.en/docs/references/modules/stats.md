---
title: "Stats Module"
weight: 30
---

# Stats Module (Simple Stats)

The Stats module provides comprehensive statistics collection, monitoring, and reporting capabilities. It collects system metrics, application statistics, and provides multiple output formats including Prometheus metrics.

## Features

- Real-time statistics collection and aggregation
- Configurable buffering for high-throughput scenarios
- Persistent storage with automatic data management
- Prometheus metrics export
- Debug endpoints for system monitoring
- File system management utilities
- Goroutine monitoring
- Memory pool statistics

## Configuration

Configure the Stats module in your YAML configuration:

```yaml
stats:
  enabled: true
  persist: true
  no_buffer: true
  include_storage_stats_in_api: true
  buffer_size: 1000
  flush_interval_ms: 1000
```

## Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable/disable statistics collection |
| `persist` | boolean | `true` | Enable persistent storage of statistics |
| `no_buffer` | boolean | `true` | Disable buffering for real-time stats |
| `include_storage_stats_in_api` | boolean | `true` | Include storage statistics in API responses |
| `buffer_size` | int | `1000` | Size of statistics buffer when buffering is enabled |
| `flush_interval_ms` | int | `1000` | Flush interval in milliseconds for buffered stats |

## API Endpoints

The Stats module provides several API endpoints for monitoring and debugging:

### Statistics Endpoints

- **GET `/stats`** - Returns current statistics in JSON format
- **GET `/stats/prometheus`** - Returns statistics in Prometheus format

Example `/stats` response:
```json
{
  "simple": {
    "requests": {
      "total": 12345,
      "success": 12000,
      "failed": 345
    },
    "system": {
      "uptime": 86400,
      "memory_usage": 134217728
    }
  }
}
```

### Debug Endpoints

- **GET `/debug/goroutines`** - Returns information about active goroutines
- **GET `/debug/pool/bytes`** - Returns memory pool usage statistics

### File Management Endpoints

- **GET `/_local/files/_list`** - Lists files in the data directory
- **GET `/_local/files/:file/_list`** - Lists contents of a specific directory
- **DELETE `/_local/files/:file`** - Deletes a specific file

## Statistics Types

The module supports two main operation types for statistics:

### Increment Operations
```go
// Increment a counter
stats.Incr("category", "metric_name", 1)
```

### Decrement Operations
```go
// Decrement a counter
stats.Decr("category", "metric_name", 1)
```

## Buffering Modes

### Real-time Mode (`no_buffer: true`)
- Statistics are updated immediately
- Lower latency for stat updates
- Recommended for most use cases

### Buffered Mode (`no_buffer: false`)
- Statistics are queued and flushed periodically
- Higher throughput for high-volume scenarios
- Uses configurable buffer size and flush interval

## Data Persistence

When persistence is enabled:
- Statistics are stored in the `stats` directory under the data path
- Data survives application restarts
- Automatic directory creation and management
- File-based storage with cleanup utilities

## Prometheus Integration

The module provides Prometheus-compatible metrics via `/stats/prometheus`:

```
# HELP requests_total Total number of requests
# TYPE requests_total counter
requests_total{category="api"} 12345

# HELP memory_usage_bytes Current memory usage in bytes
# TYPE memory_usage_bytes gauge
memory_usage_bytes 134217728
```

## Usage Examples

### Collecting Custom Statistics
```go
import "infini.sh/framework/core/stats"

// Register custom statistics
stats.Incr("user_actions", "login", 1)
stats.Incr("user_actions", "logout", 1)
stats.Incr("errors", "authentication_failed", 1)
```

### Monitoring System Health
```bash
# Get current statistics
curl http://localhost:8080/stats

# Get Prometheus metrics
curl http://localhost:8080/stats/prometheus

# Monitor goroutines
curl http://localhost:8080/debug/goroutines

# Check memory pools
curl http://localhost:8080/debug/pool/bytes
```

## Integration

The Stats module integrates with:

- **Global environment** - Uses data directory configuration
- **API module** - Provides HTTP endpoints
- **Core stats system** - Registers as the default stats provider
- **Queue system** - Uses internal queuing for buffered mode
- **File system** - Manages persistent storage

## Performance Considerations

### High-Volume Scenarios
- Use buffered mode (`no_buffer: false`) for high-volume statistics
- Increase `buffer_size` for better throughput
- Adjust `flush_interval_ms` based on latency requirements

### Memory Usage
- Monitor statistics growth and implement rotation policies
- Use file management endpoints to clean up old data
- Consider disabling persistence for temporary statistics

### Real-time Requirements
- Use real-time mode (`no_buffer: true`) for immediate updates
- Enable `include_storage_stats_in_api` for comprehensive monitoring

## Best Practices

1. **Statistics Design**: Use meaningful category and metric names
2. **Monitoring**: Regularly check `/stats` endpoint for system health
3. **Cleanup**: Use file management endpoints to maintain data hygiene
4. **Prometheus**: Leverage Prometheus integration for monitoring systems
5. **Performance**: Choose appropriate buffering mode based on workload

## Troubleshooting

- **Missing Statistics**: Verify module is enabled and properly configured
- **Performance Issues**: Check buffer settings and flush intervals
- **Storage Issues**: Verify data directory permissions and disk space
- **API Errors**: Check endpoint availability and authentication requirements
- **Memory Issues**: Monitor buffer usage and adjust settings accordingly