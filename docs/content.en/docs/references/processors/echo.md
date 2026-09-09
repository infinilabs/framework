---
title: "echo"
weight: 102
---

# echo

| Category | Scope |
|----------|-------|
| Framework | pipeline |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `message` | string |  | Message text to log or process. |

## Example

```yaml
pipeline:
  - name: demo
    processor:
      - echo:
          message: "hello world"
```
