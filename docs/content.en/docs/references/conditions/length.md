---
title: "length"
weight: 80
---

# length

| Kind | Accepts |
|------|---------|
| Value operator | one or more field/length pairs |

Tests whether the length of a field's value equals an expected integer. Works with slices, arrays, strings, maps, and channels.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| field name | int | The expected length of the field's value. |

## Examples

```yaml
length:
  tags: 3
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
