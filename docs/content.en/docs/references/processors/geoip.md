---
title: "geoip"
weight: 3005
---

# geoip

| Category | Scope |
|----------|-------|
| Enrichment | record |

The "geoip" pipeline processor: enrich the current record with geographic information looked up from a MaxMind mmdb database (GeoLite2/GeoIP2 City, Country, ASN or ISP — any of them; the decoder tolerates partial records).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | string | "client_ip" | Source field to read from. |
| `database_path` | string |  | Path to the MaxMind mmdb database. |
| `target_field` | string |  | Destination field to write the result to. |
| `languages` | liststring |  | Languages used for localized output. |
| `properties` | liststring |  | Additional static properties set on the record. |
| `ignore_missing` | bool |  | Do not fail when the source field is missing. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - for_each:
      processor:
        - geoip:
            field: client_ip
            database_path: /data/GeoLite2-City.mmdb
            target: geo
```
