---
title: "or"
weight: 110
---

# or

| Kind | Accepts |
|------|---------|
| Logical operator | a list of nested conditions |

Evaluates to `true` when any inner condition is true. Uses short-circuit evaluation — stops checking on the first `true`.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| (list) | list of conditions | Nested condition blocks; at least one must match. |

## Examples

```yaml
or:
  - equals:
      _ctx.response.status_code: 401
  - equals:
      _ctx.response.status_code: 403
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
