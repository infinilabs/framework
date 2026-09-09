---
title: "string"
weight: 1009
---

# string

| Category | Scope |
|----------|-------|
| Parsing | record |

The "string" pipeline processor: the Graylog string-function toolbox as one processor (substring, prefix/suffix/ contains tests, split/join/concat, abbreviate, regex replace).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `source` | string |  | Source field to read from. |
| `target_field` | string |  | Destination field to write the result to. |
| `substring` | listint |  | [start, end] |
| `length` | string |  | target field for char count |
| `starts_with` | string |  | Only match strings starting with this prefix. |
| `ends_with` | string |  | Only match strings ending with this suffix. |
| `contains` | string |  | Only match strings containing this substring. |
| `capitalize` | bool |  | Capitalize the resulting string. |
| `swapcase` | bool |  | Swap the case of every character. |
| `abbreviate` | int |  | Maximum string length before it is abbreviated. |
| `split` | string |  | Split the string on this separator. |
| `split_limit` | int |  | Maximum number of parts produced by the split. |
| `join` | string |  | Separator used when joining parts. |
| `concat` | liststring |  | Parts concatenated into the result. |
| `replace` | liststring |  | [regex, replacement] |
| `ignore_missing` | bool |  | Do not fail when the source field is missing. |
| `tag_on_failure` | string |  | Tag appended when processing fails. |

## Example

```yaml
processor:
  - string:
      source: "message"
      target_field: "parsed"
      substring: []
      length: "length"
      starts_with: "starts_with"
```
