---
title: "urldecode"
weight: 1012
---

# urldecode

| Category | Scope |
|----------|-------|
| Parsing | record |

The "urldecode" pipeline processor: URL-decode string fields (query-string style unescaping).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fields` | listfromTo |  | Source fields to read from. |
| `ignore_missing` | bool | false | Do not fail when the source field is missing. |
| `fail_on_error` | bool | true | Fail the record when the field cannot be decoded. |

## Example

```yaml
processor:
  - urldecode:
      fields: ["message", "tags"]
      ignore_missing: true
      fail_on_error: true
```
