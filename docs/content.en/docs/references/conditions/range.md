---
title: "range"
weight: 140
---

# range

| Kind | Accepts |
|------|---------|
| Value operator | one field and `gt`/`gte`/`lt`/`lte` bounds |

Tests whether a numeric field falls within a specified range. Supports `int`, `uint`, and `float` types. Range operators are appended to the field name with a dot separator when used programmatically; in YAML the bounds are nested under the field.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `gte` | number | Greater than or equal to (>=). |
| `gt` | number | Greater than (>). |
| `lte` | number | Less than or equal to (<=). |
| `lt` | number | Less than (<). |

## Examples

```yaml
# 200 <= status_code < 300
range:
  _ctx.response.status_code:
    gte: 200
    lt: 300

# CPU usage above 90%
range:
  proc.cpu.total_p:
    gt: 0.9
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
