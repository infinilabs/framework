---
title: "queue_output"
weight: 202
---

# queue_output

| Category | Scope |
|----------|-------|
| Queue | pipeline |

The "queue_output" pipeline processor: the chain-tail companion of "consumer". It takes the message batch the consumer exposed in the context (typically after a for_each transform chain) and appends every record onto a target queue, enabling two-stage pipelines: process on one queue, sink (e.g. bulk_indexing) from another.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `queue_name` | string |  | Queue name to consume from. |
| `message_field` | string | "messages" | Context key holding the message batch. |

## Example

```yaml
processor:
  - consumer:
      queue: staging_queue
  - for_each:
      processor:
        - mutate:
            add:
              env: prod
  - queue_output:
      queue: indexed_queue
```
