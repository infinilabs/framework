---
title: "syslog_decode"
weight: 1010
---

# syslog_decode

| Category | Scope |
|----------|-------|
| Parsing | record |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `priority` | string |  | Priority used when several entries compete. |
| `facility` | string |  | Field or facility selector used when decoding syslog codes. |
| `severity` | string |  | Severity value or field used for normalization. |
| `prefix` | string |  | Prefix tested against, or prepended to, the field. |

## Example

```yaml
processor:
  - syslog_decode:
      priority: "priority"
      facility: "facility"
      severity: "severity"
      prefix: "prefix"
```
