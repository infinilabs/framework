---
title: "consumer"
weight: 201
---

# consumer

| Category | Scope |
|----------|-------|
| Queue | pipeline |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `num_of_slices` | int | 1 | Number of slices the work is split into. |
| `slices` | listint |  | Number of parallel slices. |
| `idle_timeout_in_seconds` | int | 5 | Seconds of inactivity before a worker parks. |
| `max_connection_per_node` | int | 1 | Connection pool size per host. |
| `queues` | map |  | Queue names to consume from. |
| `queue_selector` | queue.QueueSelector |  | Label selector that picks queues by their tags (ALL pairs must match). |
| `force_queue_type` | string |  | Force the created queue to this type. |
| `consumer` | *queue.ConsumerConfig |  | Nested consumer settings (group, labels, auto_reset_offset, ...) applied on top of the shared registry config. |
| `max_worker_size` | int | 1 | Upper bound for the worker count. |
| `detect_active_queue` | bool | true | Only pick up queues that have pending messages. |
| `detect_interval` | int | 5000 | How often to probe for new queues. |
| `quite_detect_after_idle_in_ms` | int | 30000 | Stop detecting new queues after this many milliseconds of idleness. |
| `processor` | list (processor configs) |  | Ordered sub-chain of processors executed per record. |
| `skip_empty_queue` | bool | false | Advance to the next queue or segment when this one is empty. |
| `quit_on_eof_queue` | bool | true | Stop consuming when the queue reaches its end. |
| `quit_need_tag` | bool |  | need tag to quit, or wait for timeout |
| `quit_need_tag_name` | string |  | need tag to quit, or wait for timeout |
| `queue_name_field` | string | "queue_name" | Field that receives the queue name. |
| `message_field` | string | "messages" | Context key holding the message batch. |
| `waiting_after` | liststring |  | Pause after finishing a sweep before the next one. |
| `retry_delay_interval` | int | 5000 | Delay between retries. |
| `auto_commit_offset` | bool | true | Commit consumed offsets automatically. |

## Example

```yaml
processor:
  - consumer:
      queue: log_queue
      consumer:
        labels:
          topic: nginx
        auto_reset_offset: earliest
```
