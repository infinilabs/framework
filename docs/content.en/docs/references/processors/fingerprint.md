---
title: "fingerprint"
weight: 3004
---

# fingerprint

| Category | Scope |
|----------|-------|
| Enrichment | record |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `fields` | liststring |  | Source fields to read from. |
| `target` | string |  | Destination prefix or field to write the result to. |
| `method` | string | "sha256" | HTTP method of the request. |

## Example

```yaml
processor:
  - fingerprint:
      fields: ["message", "tags"]
      target: "parsed"
      method: "GET"
```
