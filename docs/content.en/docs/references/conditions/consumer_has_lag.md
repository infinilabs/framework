---
title: "consumer_has_lag"
weight: 30
---

# consumer_has_lag

| Kind | Accepts |
|------|---------|
| Domain condition | queue/group/name fields |

Tests whether a consumer group has fallen behind the producer on a queue.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `queue` | string | Queue name. |
| `group` | string | Consumer group. |
| `name` | string | Consumer name. |

## Examples

```yaml
consumer_has_lag:
  queue: "my_queue"
  group: "consumer_group"
  name: "consumer_1"
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
