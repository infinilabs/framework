---
title: "tap"
weight: 212
---

# tap

| Category | Scope |
|----------|-------|
| General | record |

Side-channel sampler: copies a sampled fraction of records to a debug queue or lane. The main flow is never affected — it is an in-flight **copy**, not a divert. Sanctioned side-channel for debug indices, shadow runs and observation sampling. Exactly one target (`queue` or `lane`).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `sample_ratio` | float | 1.0 | Fraction of records copied, within (0, 1]. `1.0` copies everything. |
| `queue` | object |  | Side target queue (`{ name }`). Mutually exclusive with `lane`. |
| `lane` | string |  | Side target lane. Mutually exclusive with `queue`. |

## Example

Copy 10% of records into a debug lane:

```yaml
- tap:
    sample_ratio: 0.1
    lane: debug-samples
```
