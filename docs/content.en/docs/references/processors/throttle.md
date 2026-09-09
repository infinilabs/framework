---
title: "throttle"
weight: 4003
---

# throttle

| Category | Scope |
|----------|-------|
| Routing | record |

The "throttle" pipeline processor: per-key rate limiting over the current records, backed by the framework's rate limiter (token bucket). Records exceeding the budget are either tagged (default) or dropped.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `key_fields` | liststring |  | Fields hashed or grouped into the rate-limit key. |
| `max_events` | int | 100 | Maximum number of events handled per invocation. |
| `per` | string | "1s" | Rate-limit window the `max_events` budget applies to (duration, e.g. `1s`). |
| `action` | string | "tag" | tag | drop |
| `tag` | string | "_throttled" | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - for_each:
      processor:
        - throttle:
            key_fields: ["host.name"]
            max_events: 100
            per: 1s
            action: tag
            tag: _throttled
```
