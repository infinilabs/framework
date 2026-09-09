---
title: "in"
weight: 70
---

# in

| Kind | Accepts |
|------|---------|
| Value operator | one field and a list of values |

Tests whether a field value is contained in a list of allowed values. Supports both string and integer values.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| field name | list | The allowed values for that field. |

## Examples

```yaml
in:
  _ctx.response.status_code: [200, 201, 204]

in:
  env: ["production", "staging"]
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
