---
title: "smtp"
weight: 503
---

# smtp

| Category | Scope |
|----------|-------|
| General | pipeline |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `dial_timeout_in_seconds` | int | 30 | Connection establishment timeout, in seconds. |
| `message_field` | param.ParaKey | "messages" | Context key holding the message batch. |
| `variable_start_tag` | string | "$[[" | Opening delimiter of template variables. |
| `variable_end_tag` | string | "]]" | Closing delimiter of template variables. |
| `variables` | map |  | Variables injected into the script or template. |
| `servers` | map[string]*ServerConfig |  | Server addresses (`host:port`). |
| `templates` | map[string]*Template |  | Templates applied when rendering output. |

## Example

```yaml
processor:
  - smtp:
      dial_timeout_in_seconds: 10
      message_field: "message_field"
      variable_start_tag: "variable_start_tag"
      variable_end_tag: "variable_end_tag"
      variables: []
```
