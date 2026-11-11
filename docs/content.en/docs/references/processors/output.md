---
title: "output"
weight: 210
---

# output

| Category | Scope |
|----------|-------|
| Output transport | batch (queue target) · record (lane target) |

Unified output step: writes the processed batch or record to **exactly one** target — a durable queue or an in-memory lane. Config unpacking rejects zero or multiple targets. `queue_output` is a registered alias for the queue form.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `queue` | string or object |  | Target queue: bare name, or `{ name, label }` object. Queue and lane are mutually exclusive. |
| `lane` | string |  | Target lane name. Record scope only — the lane carries live event pointers. Auto-created (block-on-full, default capacity) when no `lane:` stanza defines it. |
| `ack` | bool |  | Lane target only: attach an acknowledgment receipt to the context's AckSet, so the emitting consumer's commit gate waits for settlement (cross-lane at-least-once). |
| `codec` | string |  | Queue target: override the codec resolution. |
| `message_field` | string | "messages" | Batch scope (queue target): context key holding the batch. |

## Scope rules

| Target | Scope | Notes |
|--------|-------|-------|
| `queue` | batch (top level) | Encodes the batch through the 3-level codec resolution. |
| `lane` | record (inside `for_each` / `lane_consumer`) | Placing it at the consumer top level errors per batch with "requires record scope". |

## Example

Lane hand-off — the main pipeline writes processed records into an in-memory lane instead of a second disk queue (staging stays the only disk write):

```yaml
processor:
  - consumer:
      queue_selector:
        keys: ["staging-<streamID>"]
      message_field: messages
      processor:
        - for_each:
            message_field: messages
            parallel: 4
            processor:
              - grok: { field: message, patterns: ["..."] }
              - output:
                  lane: ingest-lane-<streamID>
```
