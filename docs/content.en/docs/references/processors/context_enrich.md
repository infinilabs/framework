---
title: "context_enrich"
weight: 2001
---

# context_enrich

| Category | Scope |
|----------|-------|
| Transform | record |

A processor that promotes collection context — the collecting agent's identity and the envelope's stable resource attributes — into the record's Fields.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fields` | liststring |  | Source fields to read from. |
| `ignore_missing` | bool | true | Do not fail when the source field is missing. |

## Example

```yaml
processor:
  - context_enrich:
      fields: ["message", "tags"]
      ignore_missing: true
```
