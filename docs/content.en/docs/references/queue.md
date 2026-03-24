---
title: "Queue"
weight: 20
---
# Queue

The INFINI Framework provides a pluggable queue abstraction for asynchronous message passing between components. Queues decouple producers from consumers, enabling reliable data pipelines, buffering, and background processing. The framework ships with multiple backend implementations — disk-based persistence, in-memory queues, Kafka, and Redis — all accessed through a unified API.

## Queue Interfaces

The queue system defines three levels of capability through separate interfaces in `core/queue/`.

### QueueAPI

`QueueAPI` is the base interface that every queue backend must implement. It provides lifecycle management and basic push operations.

```go
type QueueAPI interface {
    Init(cfg *QueueConfig) error
    Close(queueID string) error
    Push(queueID string, data []byte) error
    Destroy(queueID string) error
}
```

| Method | Description |
|--------|-------------|
| `Init(cfg *QueueConfig) error` | Initializes a queue from the given configuration. Creates underlying storage or connections as needed. |
| `Close(queueID string) error` | Closes the queue, flushing any pending data. The queue can be reopened later. |
| `Push(queueID string, data []byte) error` | Pushes a raw byte message onto the queue. |
| `Destroy(queueID string) error` | Permanently destroys the queue and all its data. |

### SimpleQueueAPI

`SimpleQueueAPI` extends `QueueAPI` with synchronous pop and depth inspection. Use this for straightforward single-consumer scenarios.

```go
type SimpleQueueAPI interface {
    QueueAPI
    Pop(queueID string, timeout time.Duration) (data []byte, err error)
    Depth(queueID string) int64
}
```

| Method | Description |
|--------|-------------|
| `Pop(queueID string, timeout time.Duration) ([]byte, error)` | Removes and returns the next message from the queue. Blocks up to `timeout` if the queue is empty. |
| `Depth(queueID string) int64` | Returns the number of messages currently in the queue. |

### AdvancedQueueAPI

`AdvancedQueueAPI` extends `QueueAPI` with producer/consumer lifecycle management, enabling multi-consumer patterns, offset tracking, and consumer groups.

```go
type AdvancedQueueAPI interface {
    QueueAPI
    AcquireConsumer(k *QueueConfig, consumer *ConsumerConfig, clientID string) (ConsumerAPI, error)
    ReleaseConsumer(k *QueueConfig, consumer *ConsumerConfig, clientID string) error
    AcquireProducer(k *QueueConfig) (ProducerAPI, error)
    ReleaseProducer(k *QueueConfig) error
    GetQueues() []QueueConfig
}
```

| Method | Description |
|--------|-------------|
| `AcquireConsumer(k, consumer, clientID)` | Creates or retrieves a consumer bound to the specified queue and consumer group. The `clientID` identifies this particular consumer instance. |
| `ReleaseConsumer(k, consumer, clientID)` | Releases the consumer, freeing any held resources and allowing rebalancing. |
| `AcquireProducer(k)` | Creates or retrieves a producer for the specified queue. |
| `ReleaseProducer(k)` | Releases the producer and its resources. |
| `GetQueues() []QueueConfig` | Returns the configurations of all queues managed by this backend. |

## Producer and Consumer APIs

Producers and consumers are stateful handles returned by the `AdvancedQueueAPI`. They provide a focused interface for writing and reading messages.

### ProducerAPI

```go
type ProducerAPI interface {
    Push(data []byte) error
    Close() error
}
```

| Method | Description |
|--------|-------------|
| `Push(data []byte) error` | Sends a message to the queue this producer is bound to. |
| `Close() error` | Closes the producer and releases its resources. |

### ConsumerAPI

```go
type ConsumerAPI interface {
    FetchMessages(ctx context.Context, numOfMessages int) (messages []Message, timeout bool, err error)
    CommitOffset(offset Offset) error
    Close() error
}
```

| Method | Description |
|--------|-------------|
| `FetchMessages(ctx, numOfMessages)` | Fetches up to `numOfMessages` from the queue. Returns the messages, a flag indicating whether the call timed out, and any error. The `ctx` parameter supports cancellation. |
| `CommitOffset(offset Offset) error` | Commits the consumer's read offset, marking messages as processed. This prevents redelivery after restart. |
| `Close() error` | Closes the consumer and releases its resources. |

## Queue Configuration

Queues are configured using the `QueueConfig` struct. Configuration can come from YAML files or be constructed programmatically.

```go
type QueueConfig struct {
    Source   string                 `config:"source" json:"source,omitempty"`
    ID       string                 `config:"id"     json:"id,omitempty"`
    Name     string                 `config:"name"   json:"name,omitempty"`
    Group    string                 `config:"group"  json:"group,omitempty"`
    Type     string                 `config:"type"   json:"type,omitempty"`
    Labels   map[string]interface{} `config:"label"  json:"label,omitempty"`
}
```

| Field | Type | Description |
|-------|------|-------------|
| `Source` | `string` | Identifies the origin or owning component of the queue. |
| `ID` | `string` | Unique identifier for the queue instance. Auto-generated if not provided. |
| `Name` | `string` | Human-readable name for the queue. Used in configuration references and logging. |
| `Group` | `string` | Logical grouping for related queues. Useful for organizing queues by function or tenant. |
| `Type` | `string` | Selects the queue backend implementation (e.g., `disk`, `memory`, `kafka`, `redis`). |
| `Labels` | `map[string]interface{}` | Arbitrary key-value metadata attached to the queue for filtering and categorization. |

### YAML Configuration

Queues are defined under the `queue` section in the application's YAML configuration file:

```yaml
queue:
  - name: "my_queue"
    type: "disk"
    label:
      type: "logs"

  - name: "fast_queue"
    type: "memory"
    label:
      type: "metrics"
```

## Registering Queue Backends

Queue backends register themselves using `Register` or `RegisterDefaultHandler`. Registration typically happens inside a Go `init()` function so that importing the package is sufficient to activate the backend.

```go
// Register a named queue backend
queue.Register(name string, h QueueAPI)

// Register the default handler used when no type is specified
queue.RegisterDefaultHandler(h QueueAPI)
```

### Registration Pattern

```go
package my_queue

import "infini.sh/framework/core/queue"

func init() {
    handler := &MyQueueBackend{}
    queue.Register("my_backend", handler)
}
```

Once registered, the backend can be selected by setting `type: "my_backend"` in the queue configuration.

## Producing Messages

The framework provides package-level functions for producing messages without managing producer lifecycles manually.

### Simple Push

For one-off or low-frequency writes, use the package-level `Push` function via `IniQueue` and direct push:

```go
cfg := &queue.QueueConfig{
    Name: "my_queue",
}

// Initialize the queue
err := queue.IniQueue(cfg)
if err != nil {
    log.Error("failed to initialize queue:", err)
    return
}

// Push a message
err = queue.Push(cfg, []byte("hello world"))
if err != nil {
    log.Error("failed to push message:", err)
}
```

### Using a Producer

For high-throughput scenarios, acquire a dedicated producer to amortize connection and buffer overhead:

```go
cfg := &queue.QueueConfig{
    Name: "my_queue",
}

// Acquire a producer
producer, err := queue.AcquireProducer(cfg)
if err != nil {
    log.Error("failed to acquire producer:", err)
    return
}
defer queue.ReleaseProducer(cfg)

// Push multiple messages
for i := 0; i < 1000; i++ {
    data := []byte(fmt.Sprintf("message-%d", i))
    if err := producer.Push(data); err != nil {
        log.Error("push failed:", err)
        break
    }
}
```

## Consuming Messages

Consumers read messages from a queue with offset tracking, supporting at-least-once delivery semantics.

```go
cfg := &queue.QueueConfig{
    Name: "my_queue",
}

consumerCfg := &queue.ConsumerConfig{
    Group: "my_consumer_group",
    Name:  "worker-1",
}

// Acquire a consumer
consumer, err := queue.AcquireConsumer(cfg, consumerCfg, "client-001")
if err != nil {
    log.Error("failed to acquire consumer:", err)
    return
}
defer queue.ReleaseConsumer(cfg, consumerCfg, "client-001")

// Fetch and process messages
ctx := context.Background()
for {
    messages, timeout, err := consumer.FetchMessages(ctx, 100)
    if err != nil {
        log.Error("fetch failed:", err)
        break
    }

    if timeout || len(messages) == 0 {
        continue
    }

    for _, msg := range messages {
        // Process the message
        processMessage(msg)
    }

    // Commit the offset of the last message
    lastMsg := messages[len(messages)-1]
    if err := consumer.CommitOffset(lastMsg.Offset); err != nil {
        log.Error("commit failed:", err)
    }
}
```

### Checking Consumer Lag

Use `HasLag` to determine whether a queue has unconsumed messages:

```go
cfg := &queue.QueueConfig{
    Name: "my_queue",
}

if queue.HasLag(cfg) {
    log.Info("queue has pending messages")
}
```

## Queue Backends

The framework ships with four queue backend implementations. Each backend is activated by importing its package.

### Disk Queue (Default)

The disk queue provides persistent, file-based message storage. Messages survive application restarts, making it suitable for reliable data pipelines.

**Activation:**

```go
import _ "infini.sh/framework/modules/queue/disk_queue"
```

**Configuration:**

```yaml
queue:
  - name: "persistent_queue"
    type: "disk"

disk_queue:
  max_msg_size: 20485760          # Maximum message size in bytes (default ~20MB)
  max_bytes_per_file: 209715200   # Maximum segment file size (default ~200MB)
  sync_every_records: 10000       # Sync to disk every N records
  retention:
    max_num_of_local_files: 20    # Maximum number of segment files to retain
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `max_msg_size` | `int` | Maximum allowed size for a single message in bytes. Messages exceeding this limit are rejected. |
| `max_bytes_per_file` | `int` | Maximum size of each segment file on disk. When reached, a new segment file is created. |
| `sync_every_records` | `int` | Number of records to write before forcing a disk sync. Lower values increase durability at the cost of throughput. |
| `retention.max_num_of_local_files` | `int` | Maximum number of segment files to keep. Older segments are removed when this limit is exceeded. |

### Memory Queue

The memory queue stores messages in RAM for maximum throughput. Messages are lost on application restart. Use this for transient data, caching, or scenarios where speed matters more than durability.

**Activation:**

```go
import _ "infini.sh/framework/modules/queue/mem_queue"
```

**Configuration:**

```yaml
queue:
  - name: "fast_queue"
    type: "memory"
```

### Kafka Queue

The Kafka backend delegates to an external Apache Kafka cluster. Use this when you need distributed messaging, replication, and integration with the broader Kafka ecosystem.

**Activation:**

```go
import _ "infini.sh/framework/modules/queue/kafka_queue"
```

**Configuration:**

```yaml
queue:
  - name: "distributed_queue"
    type: "kafka"
```

### Redis Queue

The Redis backend uses Redis as the message broker. Use this for lightweight distributed queuing when a Redis instance is already available.

**Activation:**

```go
import _ "infini.sh/framework/modules/queue/redis"
```

**Configuration:**

```yaml
queue:
  - name: "redis_queue"
    type: "redis"
```

## Complete Example

Below is a complete example demonstrating queue initialization, producing, and consuming messages using the disk queue backend.

### Application Setup

```go
package main

import (
    "context"
    "fmt"
    "time"

    log "github.com/cihub/seelog"
    "infini.sh/framework/core/queue"
    _ "infini.sh/framework/modules/queue/disk_queue"
)

func main() {
    // Define queue configuration
    cfg := &queue.QueueConfig{
        Name: "example_queue",
        Type: "disk",
        Labels: map[string]interface{}{
            "type": "demo",
        },
    }

    // Initialize the queue
    if err := queue.IniQueue(cfg); err != nil {
        log.Errorf("failed to init queue: %v", err)
        return
    }

    // Start a producer goroutine
    go produce(cfg)

    // Start a consumer
    consume(cfg)
}

func produce(cfg *queue.QueueConfig) {
    producer, err := queue.AcquireProducer(cfg)
    if err != nil {
        log.Errorf("failed to acquire producer: %v", err)
        return
    }
    defer queue.ReleaseProducer(cfg)

    for i := 0; i < 100; i++ {
        msg := []byte(fmt.Sprintf(`{"event": "click", "seq": %d}`, i))
        if err := producer.Push(msg); err != nil {
            log.Errorf("push error: %v", err)
            return
        }
    }
    log.Info("finished producing 100 messages")
}

func consume(cfg *queue.QueueConfig) {
    consumerCfg := &queue.ConsumerConfig{
        Group: "demo_group",
        Name:  "demo_consumer",
    }

    consumer, err := queue.AcquireConsumer(cfg, consumerCfg, "client-1")
    if err != nil {
        log.Errorf("failed to acquire consumer: %v", err)
        return
    }
    defer queue.ReleaseConsumer(cfg, consumerCfg, "client-1")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    total := 0
    for {
        messages, timeout, err := consumer.FetchMessages(ctx, 10)
        if err != nil {
            log.Errorf("fetch error: %v", err)
            break
        }

        if timeout {
            log.Info("fetch timed out, stopping consumer")
            break
        }

        for _, msg := range messages {
            fmt.Printf("received: %s\n", string(msg.Data))
            total++
        }

        if len(messages) > 0 {
            last := messages[len(messages)-1]
            if err := consumer.CommitOffset(last.Offset); err != nil {
                log.Errorf("commit error: %v", err)
            }
        }
    }

    log.Infof("consumed %d messages total", total)
}
```

### YAML Configuration

```yaml
queue:
  - name: "example_queue"
    type: "disk"
    label:
      type: "demo"

disk_queue:
  max_msg_size: 20485760
  max_bytes_per_file: 209715200
  sync_every_records: 10000
  retention:
    max_num_of_local_files: 20

pipeline:
  - name: queue_demo
    auto_start: true
    keep_running: false
    processor:
      - echo:
          message: "queue demo pipeline started"
```
