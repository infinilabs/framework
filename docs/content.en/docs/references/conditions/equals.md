---
title: "equals"
weight: 50
---

# equals

| Kind | Accepts |
|------|---------|
| Value operator | one or more field/equality pairs |

Tests whether a field value is exactly equal to an expected value. Supports `string`, `int`, `float`, and `bool` types. When multiple fields are specified, all must match (implicit AND).

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| field name | any scalar | The expected value for that field. |

Multiple entries are combined with AND; every listed field must equal its expected value.

## Examples

```yaml
equals:
  _ctx.request.method: "GET"

equals:
  type: "process"
  proc.pid: 305
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
