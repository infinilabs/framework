---
title: "drop_fields"
weight: 5003
---

# drop_fields

| Category | Scope |
|----------|-------|
| Governance | record |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fields` | liststring |  | Source fields to read from. |
| `keep` | bool |  | Invert into prune semantics: keep only the listed fields. |

## Example

```yaml
processor:
  - drop_fields:
      fields: ["message", "tags"]
      keep: true
```
