---
title: "field_standardize"
weight: 2004
---

# field_standardize

| Category | Scope |
|----------|-------|
| Transform | record |

The "field_standardize" pipeline processor: it normalizes the attribute keys of the current record to the canonical naming of the INFINI log data model (OTel Log Data Model with lowercase snake_case keys by default).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `mode` | string |  | Operating mode of the processor. |
| `normalize_case` | bool | true | Normalize the case of the mapped keys. |
| `flatten` | *bool |  | Flatten all array elements as indexed fields. |
| `separator` | string |  | Separator between keys and values, or between fields. |
| `mapping` | map (string to string) |  | Field mapping applied by the processor. |
| `drop_unknown` | bool |  | Drop fields that have no mapping. |

## Example

```yaml
processor:
  - field_standardize:
      mode: "mode"
      normalize_case: true
      flatten: "flatten"
      separator: "="
      mapping: []
```
