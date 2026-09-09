---
title: "drop_event"
weight: 5002
---

# drop_event

| Category | Scope |
|----------|-------|
| Governance | record |

The "drop_event" pipeline processor: mark the current record to be dropped from the batch. for_each honors the mark and removes the record's payload before the batch is forwarded, so downstream stages (otlp_export etc.) never see it.

## Example

```yaml
processor:
  - drop_event:
      {}
```
