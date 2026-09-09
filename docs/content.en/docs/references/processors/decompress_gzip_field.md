---
title: "decompress_gzip_field"
weight: 1005
---

# decompress_gzip_field

| Category | Scope |
|----------|-------|
| Parsing | record |

The "decompress_gzip_field" pipeline processor: gunzip a string or bytes field in place (e.g. a compressed payload collected from a source system).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | fromTo |  | Source field to read from. |
| `ignore_missing` | bool | false | Do not fail when the source field is missing. |
| `fail_on_error` | bool | true | Fail the record when the field cannot be decoded. |

## Example

```yaml
processor:
  - decompress_gzip_field:
      field: "message"
      ignore_missing: true
      fail_on_error: true
```
