---
title: "Queue Module"
weight: 60
---

# Queue Module

The Queue module provides message queue management capabilities with support for multiple queue types including disk-based and memory-based queues. It handles queue metadata persistence and provides a centralized queue management system.

## Features

- Multiple queue backend support (disk and memory)
- Queue metadata management and persistence
- Configurable queue creation and management
- Queue monitoring and statistics
- Automatic queue cleanup and maintenance
- Plugin-based architecture for queue implementations

## Configuration

Configure queues in your YAML configuration:

```yaml
queue:
  - id: "default"
    name: "default"
    type: "disk"
    enabled: true
    settings:
      path: "./data/queues/default"
      max_bytes: 104857600  # 100MB
      sync_every: 1000
      
  - id: "memory_queue"
    name: "memory_queue" 
    type: "memory"
    enabled: true
    settings:
      max_items: 10000
      max_bytes: 52428800  # 50MB
      
  - id: "high_priority"
    name: "high_priority"
    type: "disk"
    enabled: true
    settings:
      path: "./data/queues/priority"
      max_bytes: 209715200  # 200MB
      sync_every: 100  # More frequent sync for priority
```

## Configuration Parameters

### Queue Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `id` | string | Required | Unique queue identifier |
| `name` | string | Required | Queue name (defaults to id if not specified) |
| `type` | string | `"disk"` | Queue type (disk, memory) |
| `enabled` | boolean | `true` | Enable/disable the queue |
| `settings` | object | `{}` | Queue-specific configuration |

### Disk Queue Settings

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `path` | string | Required | Directory path for queue data |
| `max_bytes` | int64 | `104857600` | Maximum queue size in bytes |
| `sync_every` | int | `1000` | Sync to disk every N operations |
| `max_msg_size` | int | `1048576` | Maximum message size |
| `segment_size` | int64 | `10485760` | Queue segment size |

### Memory Queue Settings

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_items` | int | `10000` | Maximum number of items in memory |
| `max_bytes` | int64 | `52428800` | Maximum memory usage in bytes |
| `overflow_to_disk` | boolean | `false` | Overflow to disk when memory full |

## Queue Types

### Disk Queue
- **Persistent storage** - Messages survive application restarts
- **Large capacity** - Limited only by available disk space  
- **Durability** - Configurable sync frequency for durability vs performance
- **Segments** - Uses multiple files for efficient management

### Memory Queue
- **High performance** - Fastest queue operations
- **Limited capacity** - Constrained by available memory
- **Volatile** - Messages lost on application restart
- **Overflow handling** - Optional disk overflow support

## Queue Operations

### Basic Operations

```go
import "infini.sh/framework/core/queue"

// Get a queue instance
q := queue.GetOrInitConfig("default")

// Push a message
err := q.Push([]byte("message data"))
if err != nil {
    log.Error("Failed to push message:", err)
}

// Pop a message
message, err := q.Pop()
if err != nil {
    log.Error("Failed to pop message:", err)
}

// Get queue depth
depth := q.Depth()
log.Info("Queue depth:", depth)
```

### Batch Operations

```go
// Push multiple messages
messages := [][]byte{
    []byte("message 1"),
    []byte("message 2"), 
    []byte("message 3"),
}

for _, msg := range messages {
    q.Push(msg)
}

// Pop with timeout
message, err := q.PopWithTimeout(5 * time.Second)
if err != nil {
    if err == queue.ErrTimeout {
        log.Info("No message available within timeout")
    } else {
        log.Error("Pop error:", err)
    }
}
```

## Queue Metadata

The Queue module automatically manages metadata for all configured queues:

### Metadata Storage
- Queue configuration and state information
- Performance statistics and metrics
- Queue health and status data
- Automatic persistence on shutdown

### Metadata Operations
```go
// Get queue metadata
metadata := queue.GetQueueMetadata("default")
if metadata != nil {
    log.Info("Queue depth:", metadata.Depth)
    log.Info("Total messages:", metadata.TotalMessages)
}

// List all queues
queues := queue.ListQueues()
for _, queueName := range queues {
    log.Info("Available queue:", queueName)
}
```

## Performance Tuning

### Disk Queue Optimization
```yaml
queue:
  - name: "high_performance"
    type: "disk"
    settings:
      path: "/fast-ssd/queues/high_perf"
      sync_every: 10000    # Less frequent sync for performance
      segment_size: 104857600  # Larger segments
      max_bytes: 1073741824    # 1GB capacity
```

### Memory Queue Optimization
```yaml
queue:
  - name: "fast_memory"
    type: "memory"
    settings:
      max_items: 100000      # Higher capacity
      max_bytes: 268435456   # 256MB
      overflow_to_disk: true # Fallback to disk
```

## Monitoring and Statistics

### Queue Metrics
- Message throughput (messages/second)
- Queue depth and capacity utilization
- Push/pop operation latencies
- Error rates and failed operations

### Health Monitoring
```go
// Check queue health
healthy := queue.IsHealthy("default")
if !healthy {
    log.Warn("Queue health check failed")
}

// Get queue statistics
stats := queue.GetStats("default")
log.Info("Messages processed:", stats.TotalProcessed)
log.Info("Current depth:", stats.CurrentDepth)
log.Info("Average latency:", stats.AvgLatency)
```

## Integration

The Queue module integrates with:

- **Core queue system** - Provides queue implementations
- **Stats module** - Reports queue performance metrics
- **Pipeline system** - Processes messages from queues
- **Global environment** - Uses data directory configuration

## Use Cases

### Message Buffering
```yaml
# Buffer incoming messages during high load
queue:
  - name: "input_buffer"
    type: "disk"
    settings:
      path: "./data/buffer"
      max_bytes: 1073741824  # 1GB buffer
```

### Task Processing
```yaml
# Queue for background task processing
queue:
  - name: "task_queue"
    type: "disk"
    settings:
      path: "./data/tasks"
      sync_every: 100  # Frequent sync for reliability
```

### Event Streaming
```yaml
# High-throughput event processing
queue:
  - name: "events"
    type: "memory"
    settings:
      max_items: 50000
      overflow_to_disk: true
```

## Best Practices

1. **Choose appropriate queue type** - Use disk for persistence, memory for speed
2. **Size queues properly** - Consider message size and throughput requirements
3. **Monitor queue depth** - Prevent unbounded queue growth
4. **Use multiple queues** - Separate different types of messages
5. **Configure sync frequency** - Balance durability vs performance needs

## Troubleshooting

### Common Issues

1. **Queue startup failures**
   - Check directory permissions for disk queues
   - Verify available disk space
   - Review queue configuration syntax

2. **Performance problems**  
   - Adjust sync frequency for disk queues
   - Consider memory queues for high-throughput scenarios
   - Monitor disk I/O for bottlenecks

3. **Memory issues**
   - Monitor memory queue usage
   - Enable overflow to disk for memory queues
   - Adjust max_items and max_bytes limits

### Debug Configuration
```yaml
queue:
  - name: "debug_queue"
    type: "disk"
    settings:
      debug: true          # Enable debug logging
      log_operations: true # Log all operations
```

### Recovery Procedures

For disk queue corruption:
1. Stop the application
2. Check queue segment files in the data directory  
3. The queue system includes automatic recovery capabilities
4. Monitor startup logs for recovery progress