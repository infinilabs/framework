---
title: "if"
weight: 207
---

# if

| Category | Scope |
|----------|-------|
| Routing | record (sub-chain host) |

Conditional branch: when the condition matches, records run the `then` sub-chain; otherwise the `else` sub-chain. After the branch completes, the event returns to the main chain. The condition body is written directly under the `if` key; `then`/`else` are sibling keys holding processor objects.

Conditions use the [conditions DSL](../conditions/) — field comparisons, patterns, ranges, network membership, and logical `and`/`or`/`not` combinations, nestable arbitrarily.

## Structure

```yaml
- if:
    <condition>            # any condition operator, see Conditions
  then:
    - <processor>...       # runs when the condition matches
  else:
    - <processor>...       # optional; runs when it does not
```

| Key | Description |
|-----|-------------|
| `<condition>` | Condition operator at the `if` key (`equals`, `regexp`, `range`, `and`, ...). |
| `then` | Sub-chain for matching records. An empty `then` fails compile validation. |
| `else` | Optional sub-chain for non-matching records. |
| `when` | Per-processor conditional execution is the same mechanism declared on any processor (evaluates the condition, skips the step when it does not match). |

## Example

```yaml
- if:
    equals:
      log_level: error
  then:
    - drop_event:
  else:
    - mutate:
        add:
          routed: non-error
```

## Notes

- Branch execution returns to the main chain afterwards; a branch `terminal` processor (e.g. [`drop_event`](drop_event/)) ends the event instead.
- Sub-chain processors must be safe for concurrent `Process` calls when the surrounding [`for_each`](for_each/) runs with `parallel > 1` (each processor only touches its own record).
