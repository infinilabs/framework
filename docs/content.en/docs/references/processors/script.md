---
title: "script"
weight: 7001
---

# script

| Category | Scope |
|----------|-------|
| Advanced | record |

The "script" pipeline processor: arbitrary record transforms in ECMAScript (goja) — the escape hatch when the declarative processors cannot express the logic.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `script` | string |  | ECMAScript (goja) source of the transform. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - script:
      script: "script"
      ignore_failure: true
      tag: "tag"
```
