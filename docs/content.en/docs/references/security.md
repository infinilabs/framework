---
title: "Security & Authentication"
weight: 15
---

# Security & Authentication

The framework provides a pluggable authentication and authorization layer for
the Web server. It supports multiple authentication backends (static, native,
access tokens, OAuth) that can be combined, a unified password-login endpoint,
and a permission-based authorization system with statically-defined roles.

All security configuration lives under `web.security`:

```yaml
web:
  security:
    enabled: true            # master switch — when false, all auth is bypassed
    managed: false           # true = user/role management is external (read-only here)
    authentication: { ... }  # identity verification (who you are)
    authorization:   { ... } # permission checks (what you can do)
```

> Authentication and authorization are configured independently. `authentication`
> answers "who is this user?"; `authorization` answers "what are they allowed to
> do?". A static user list (authentication) is typically paired with static role
> definitions (authorization).

---

## Authentication

Configured under `web.security.authentication`. Multiple backends can be
enabled at once; they are tried in order until one recognizes the credential.

```yaml
web:
  security:
    authentication:
      native:      { enabled: true }            # ORM-backed user store
      access_token: { enabled: true }           # long-lived API tokens (X-API-TOKEN)
      http_basic:  { enabled: false }           # external basic-auth gateway (not yet for web)
      oauth:                                       # OAuth providers (google, github, ...)
        google: { ... }
      static:                                     # inline user list — see below
        enabled: true
        users: [ ... ]
```

### Static authentication (inline users)

The `static` backend lets you declare users directly in the config file (or via
environment-overridden config). It is the simplest way to bootstrap accounts
without an ORM/native user store, and is the default for single-binary
deployments.

```yaml
web:
  security:
    authentication:
      static:
        enabled: true
        users:
          - id: admin                # optional; defaults to `login`
            name: Administrator       # display name
            login: admin@example.com  # username or email — the login key
            password: "Admin@123"     # PLAINTEXT — auto-hashed at startup (see note)
            roles: [admin]            # role names; resolved by authorization.static
          - id: readonly
            login: viewer@example.com
            password: "Viewer@123"
            roles: [viewer]
```

#### `users[]` fields

| Field     | Required | Description                                                                 |
|-----------|----------|-----------------------------------------------------------------------------|
| `login`   | yes      | Username or email. The unique login key used at `/account/login`.           |
| `password`| yes*     | Plaintext. Automatically bcrypt-hashed (`bcrypt.DefaultCost`) at startup.   |
| `roles`   | no       | List of role names granted to this user (resolved by authorization).        |
| `id`      | no       | Stable user ID. Defaults to `login` when omitted.                           |
| `name`    | no       | Human-readable display name.                                                |

\* A user without `password` cannot log in via password (password-less service
account).

#### Password handling — important

The `password` value is **plaintext in the config**. At startup the static
module hashes each one with bcrypt (`bcrypt.GenerateFromPassword`) and stores
only the hash in memory; the original plaintext is never retained. Login then
verifies with `bcrypt.CompareHashAndPassword`.

Because the config still carries plaintext, **prefer referencing secrets from
the keystore** rather than committing them:

```yaml
users:
  - login: admin@example.com
    password: $[[keystore.admin_password]]   # decrypted at load time
    roles: [admin]
```

See [Keystore]({{< relref "keystore" >}}) for managing secret values.

### Other backends

- **`native`** — users/roles persisted via ORM (Elasticsearch). Enables the
  in-app user-management UI. Pair with `authorization.native`.
- **`access_token`** — issues and validates long-lived API tokens sent via the
  `X-API-TOKEN` header. Managed through `POST /auth/access_token` (requires
  login first). `native: true` (default when native realm is on) persists
  tokens via ORM; `native: false` is KV-only.
- **`oauth`** — external providers (`google`, `github`, ...). Each entry needs
  at minimum `type`, `client_id`, `client_secret`, `url`.
- **`http_basic`** — delegates basic-auth to an external endpoint. Not yet
  wired into the web stack; use API basic auth or OAuth instead.

---

## Unified login endpoint — `POST /account/login`

When at least one **password-based** backend (`static` or `native`) is enabled,
the framework registers a shared login endpoint:

```http
POST /account/login
Content-Type: application/json

{"login": "admin@example.com", "password": "Admin@123"}
```

```http
HTTP/1.1 200 OK
Set-Cookie: session_token=<jwt>; HttpOnly; ...
Content-Type: application/json

{"access_token": "<jwt>", "expire_in": 1786559999, "status": "ok"}
```

**Behavior:**

- Looks up the user across all registered backends via `security.GetUserByLogin`
  (static, native, ...) and verifies the password with bcrypt.
- On success, creates a JWT session (24h) stored in the `session_token` cookie.
  Subsequent requests are authenticated by the session-token auth filter.
- Rate-limited: 10 attempts/minute per client IP. Returns `429` when exceeded.
- Errors: `400` (bad body), `401` (invalid credentials), `429` (rate limited).

**Route gating:** the endpoint is registered only when `static.enabled` or
`native.enabled` is true (checked after config load, via
`RegisterFuncBeforeSetup`). Deployments using only OAuth/access-token never
register `/account/login`, so they expose no password-bruteforce surface.

Logout: `POST /account/logout` (or `GET`) clears the session.

---

## Authorization

Configured under `web.security.authorization`. The static provider declares
roles and their permissions inline; a user's effective permissions are the
union of the permissions of all roles assigned to them (via the authentication
side `users[].roles`, or `role_mapping`).

```yaml
web:
  security:
    authorization:
      static:
        enabled: true
        roles:
          - name: admin
            permissions:
              - "*"                   # wildcard — grants everything
          - name: viewer
            permissions:
              - "generic:entity:card:read"
              - "logpilot#stream:read"
              - "logpilot#pattern:read"
        role_mapping:                 # map a login/subject → role(s)
          admin@example.com: [admin]
          viewer@example.com: [viewer]
```

### `roles[]`

| Field        | Description                                                              |
|--------------|--------------------------------------------------------------------------|
| `name`       | Role name. Referenced by `users[].roles` and `role_mapping`.             |
| `permissions`| List of permission keys. `*` grants all permissions (superuser).         |

### `role_mapping`

Optional. Maps a subject (typically a login or external identity) to one or
more role names. This is useful when roles come from an external/OAuth
identity and you want to translate them to local role names.

### Permission keys

A permission key is an opaque string the application defines and checks via
`api.RequirePermission(...)`. Conventions vary by app — e.g.
`"<scope>:<resource>:<action>"` or `"<scope>#<resource>/<action>"`. The
framework itself only compares strings; the application registers the
meaningful keys (see [API & Web Framework]({{< relref "api_web" >}}) for how
handlers attach permission requirements).

---

## Complete example

A typical static-only deployment with login, role-based permissions, and API
tokens:

```yaml
web:
  security:
    enabled: true
    managed: false
    authentication:
      static:
        enabled: true
        users:
          - id: admin
            name: Administrator
            login: admin@infini.labs
            password: $[[keystore.admin_password]]
            roles: [admin]
          - id: operator
            login: operator@infini.labs
            password: $[[keystore.operator_password]]
            roles: [operator]
      access_token:
        enabled: true
    authorization:
      static:
        enabled: true
        roles:
          - name: admin
            permissions: ["*"]
          - name: operator
            permissions:
              - "logpilot#stream/read"
              - "logpilot#stream/create"
              - "logpilot#pattern/read"
              - "logpilot#ai/admin"
        role_mapping:
          admin@infini.labs: [admin]
          operator@infini.labs: [operator]
```

With this config:

1. `POST /account/login` accepts `admin@infini.labs` / the keystore password
   (static backend verifies the bcrypt-hashed value).
2. The session gets role `admin` → all permissions.
3. `operator` can read/create streams, read patterns, and manage AI config,
   but cannot delete streams (no `logpilot#stream/delete`).
4. Both users can mint API tokens via `POST /auth/access_token` for
   headless/CLI use.

---

## Env-var overrides

AI/security-relevant secrets can be supplied through the keystore
(`$[[keystore.xxx]]`) or environment variables processed by the app's config
loader. The static backend reads from the merged config, so any config-source
override (file, env, keystore) is honored — write secrets once in the keystore
and reference them everywhere.
