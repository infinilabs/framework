---
title: "pattern_tagger"
weight: 8002
---

# pattern_tagger

| Category | Scope |
|----------|-------|
| Record | record |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `logpilot_url` | string |  | LogPilot API base, e.g. http://127.0.0.1:29000 |
| `api_token` | string |  | optional X-API-TOKEN for protected deployments |
| `stream_id` | string |  | whose merged pattern set to fetch |
| `field` | string | "message" | source field containing the raw line (default message) |
| `max_distance` | float64 | 0.6 | 0..1 (default 0.6, same as the plugin) |
| `ignore_missing` | bool | true | field absent => no-op (default true) |
| `max_field_length` | int | 10000 | guard rail; longer lines are "skipped" (default 10000, <=0 disables) |
| `resolve_top_level` | bool | true | map matched leaf to top-level ancestor (default true) |
| `refresh_interval` | string | "60s" | snapshot refresh cadence (default 60s) |
| `request_timeout` | string |  | HTTP timeout per fetch (default 15s) |
| `target_pattern_id` | string | "@pattern_id" | Field receiving the matched pattern's identifier. |
| `target_pattern_hash` | string | "@pattern_hash" | Field receiving the matched pattern's hash. |
| `target_pattern_distance` | string | "@pattern_distance" | Field receiving the matched pattern's edit distance. |
| `target_pattern_score` | string | "@pattern_score" | Field receiving the matched pattern's score. |
| `target_pattern_severity` | string | "@pattern_severity" | Field receiving the matched pattern's severity. |
| `target_pattern_status` | string | "@pattern_status" | Field receiving the matched pattern's status. |

## Example

```yaml
processor:
  - pattern_tagger:
      logpilot_url: "logpilot_url"
      api_token: "api_token"
      stream_id: "stream_id"
      field: "message"
      max_distance: 10
```
