---
title: "prefix"
weight: 120
---

# prefix

| Kind | Accepts |
|------|---------|
| Value operator | exactly one field/prefix pair |

Tests whether a string field starts with a given prefix. Accepts exactly one field.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| field name | string | The prefix the field must start with. |

## Examples

```yaml
prefix:
  hostname: "prod-"
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
