---
title: "and"
weight: 10
---

# and

| Kind | Accepts |
|------|---------|
| Logical operator | a list of nested conditions |

Evaluates to `true` only when all inner conditions are true. Uses short-circuit evaluation — stops checking on the first `false`.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| (list) | list of conditions | Nested condition blocks; every one must match. |

## Examples

```yaml
and:
  - equals:
      _ctx.request.method: "POST"
  - contains:
      _ctx.request.uri: "/api"
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
