---
title: "regexp"
weight: 150
---

# regexp

| Kind | Accepts |
|------|---------|
| Value operator | one or more field/pattern pairs |

Tests whether a string field matches a regular expression pattern. Also supports matching against arrays of strings.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| field name | string | Regular expression the field must match. |

## Examples

```yaml
regexp:
  source: "apache2/error.*"

regexp:
  message: "[Ee]rror|[Ff]ailed"
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
