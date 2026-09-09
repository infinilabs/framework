---
title: "exists"
weight: 60
---

# exists

| Kind | Accepts |
|------|---------|
| Value operator | a list of field names |

Tests whether one or more fields exist and are non-empty. Accepts a list of field names; all fields must exist for the condition to match.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| (list) | list of strings | Field names that must all exist and be non-empty. |

## Examples

```yaml
exists:
  - username
  - email
  - session_id
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
