---
title: "contains"
weight: 40
---

# contains

| Kind | Accepts |
|------|---------|
| Value operator | one or more field/substring pairs |

Tests whether a string field contains a given substring. Also works on arrays of strings — returns `true` if any element contains the substring.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| field name | string | The substring the field must contain. |

Multiple entries are combined with AND.

## Examples

```yaml
contains:
  _ctx.request.uri: "/api"

contains:
  file.path: "nginx"
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
