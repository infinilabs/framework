---
title: "http"
weight: 501
---

# http

| Category | Scope |
|----------|-------|
| General | pipeline |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `message_field` | param.ParaKey | "messages" | Context key holding the message batch. |
| `schema` | string |  | support variable |
| `hosts` | liststring |  | support variable |
| `method` | string |  | support variable |
| `path` | string |  | support variable |
| `headers` | map (string to string) |  | support variable |
| `basic_auth` | *model.BasicAuth |  | support variable |
| `tls` | *config.TLSConfig |  | client tls config |
| `compress` | bool | false | compress request body, default false |
| `compression_threshold` | int |  | default 1024 bytes |
| `valid_status_code` | listint |  | validated status code, default 200 |
| `max_sending_qps` | int |  | Rate limit for outgoing requests. |
| `max_connection_per_node` | int |  | Connection pool size per host. |
| `max_response_size` | int |  | Maximum accepted response body size. |
| `max_retry_times` | int |  | Maximum number of retries. |
| `retry_delay_in_ms` | int |  | Delay between retries, in milliseconds. |
| `max_conn_wait_timeout` | duration |  | How long a request waits for a free connection. |
| `max_idle_conn_duration` | duration |  | Maximum idle time of a pooled connection. |
| `max_conn_duration` | duration |  | Maximum lifetime of a pooled connection. |
| `timeout` | duration |  | Overall request timeout. |
| `read_timeout` | duration |  | Timeout for reading the response body. |
| `write_timeout` | duration |  | Timeout for writing the request body. |
| `read_buffer_size` | int |  | Read buffer size of the connection. |
| `write_buffer_size` | int |  | Write buffer size of the connection. |

## Example

```yaml
processor:
  - http:
      request:
        method: POST
        url: "https://example.com/api/report"
        body: '{"ok":true}'
```
