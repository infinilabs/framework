---
title: "queue_has_lag"
weight: 130
---

# queue_has_lag

| Kind | Accepts |
|------|---------|
| Domain condition | a list of queue specifiers |

Tests whether a message queue has unconsumed messages. An optional `> max_depth` threshold can be appended to a queue specifier.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| (list) | list of strings | Queue names, optionally `"queue > threshold"`. |

## Examples

```yaml
queue_has_lag:
  - "my_queue"
  - "my_queue > 1000"
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
