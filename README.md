# INFINI Framework

## Requirements
- Go 1.21+

## Dependencies

All dependencies are managed via Go modules (`go.mod`). There is no external vendor repository required.

### Internalized Libraries

The following forked/patched libraries live under `lib/` as part of this module:

| Directory | Origin | Notes |
|-----------|--------|-------|
| `lib/seelog` | `github.com/cihub/seelog` | Logging backend; exposed via `replace` directive so existing imports keep working |
| `lib/statsd` | `github.com/quipo/statsd` | StatsD client with custom buffering changes |
| `lib/gomail` | `gopkg.in/gomail.v2` | SMTP client with `NewDialerWithTimeout` extension |
| `lib/tencentcloud` | `github.com/libdns/tencentcloud` | DNS provider with custom signer and types |

### Building

```bash
make build          # production build
make build-dev      # development build (includes -tags dev)
```

No `GOPATH` manipulation or vendor repository checkout is needed. Standard `go build` works directly.