---
title: "cidr"
weight: 5001
---

# cidr

| Category | Scope |
|----------|-------|
| Governance | record |

The "cidr" pipeline processor: test an IP field against a list of networks (Vector's cidr_contains equivalent) and record the verdict — the natural gate before geoip ("skip private addresses") or routing.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | string | "client_ip" | Source field to read from. |
| `networks` | liststring |  | Networks (CIDR or named) tested against the IP field. |
| `target` | string | "cidr_matched" | Destination prefix or field to write the result to. |
| `tag_on_match` | string |  | Tag appended when the pattern matches. |

## Example

```yaml
processor:
  - cidr:
      field: "message"
      networks: []
      target: "parsed"
      tag_on_match: "tag_on_match"
```
