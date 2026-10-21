---
title: "for_each"
weight: 103
---

# for_each

| Category | Scope |
|----------|-------|
| Framework | batch |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `message_field` | string | "messages" | Context key holding the message batch. |
| `parallel` | int | 1 | Runs the record sub-chain with N workers concurrently. Log workloads are order-insensitive — with a serial per-record cost of ~330µs, `parallel: 4` measured ~2× end-to-end throughput. Processors must be safe for concurrent `Process` calls on **distinct** records (they mutate the record, never shared state); every in-repo record processor satisfies this. Cancellation still propagates from the parent context. |
| `codec` | string |  | Payload codec used to decode/encode records (default `otel`). |
| `on_failure` | string |  | Sub-chain failure strategy: `ignore`, `tag` or `fail`. |
| `failure_tag` | string | "_processing_failed" | Tag appended to the record when `on_failure` is `tag`. |
| `processor` | list (processor configs) |  | Ordered sub-chain of processors executed per record. |

## Example

```yaml
processor:
  - for_each:
      codec: otel
      on_failure: tag
      processor:
        - dissect:
            pattern: "%{log_level} %{message}"
        - if:
            equals:
              log_level: error
          then:
            - drop_event:
```
