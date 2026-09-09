---
title: "cluster_available"
weight: 20
---

# cluster_available

| Kind | Accepts |
|------|---------|
| Domain condition | a list of cluster IDs |

Tests whether one or more Elasticsearch/Easysearch clusters are available.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| (list) | list of strings | Registered cluster IDs; all must be available. |

## Examples

```yaml
cluster_available:
  - "primary_cluster"
  - "backup_cluster"
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
