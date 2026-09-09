---
title: "decode_base64_field"
weight: 1004
---

# decode_base64_field

| Category | Scope |
|----------|-------|
| Parsing | record |

The "decode_base64_field" pipeline processor: base64-decode a single string field (with or without padding).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | fromTo |  | Source field to read from. |
| `ignore_missing` | bool | false | Do not fail when the source field is missing. |
| `fail_on_error` | bool | true | Fail the record when the field cannot be decoded. |

## Example

```yaml
processor:
  - decode_base64_field:
      field: "message"
      ignore_missing: true
      fail_on_error: true
```
