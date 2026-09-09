---
title: "suffix"
weight: 160
---

# suffix

| Kind | Accepts |
|------|---------|
| Value operator | exactly one field/suffix pair |

Tests whether a string field ends with a given suffix. Accepts exactly one field.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| field name | string | The suffix the field must end with. |

## Examples

```yaml
suffix:
  filename: ".log"
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
