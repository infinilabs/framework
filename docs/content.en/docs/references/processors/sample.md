---
title: "sample"
weight: 4002
---

# sample

| Category | Scope |
|----------|-------|
| Routing | record |

The "sample" pipeline processor: probabilistic sampling — keep each record with probability ratio (0.0-1.0), drop the rest. Combine with the framework's when: conditions to sample selectively (e.g. only DEBUG logs).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ratio` | float64 | 0.5 | Keep probability for sampling (0.0 to 1.0). |

## Example

```yaml
processor:
  - for_each:
      processor:
        - sample:
            ratio: 0.1
            action: drop
```
