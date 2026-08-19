---
title: "Reverse Websocket Channel"
weight: 75
---

# Reverse Websocket Channel

The framework's reverse channel lets a control plane (e.g. INFINI Console) call HTTP APIs on agents that sit behind NAT or firewalls — without opening any inbound port on the agent side. The connection direction is inverted: the agent dials out and holds a long-lived websocket; the control plane pushes requests down that connection and the agent executes them locally.

```
┌────────────┐  ① agent dials out (outbound-only)   ┌─────────────────┐
│   Agent    │ ──────────────────────────────────▶  │  Control Plane   │
│ (NAT/fire- │      long-lived websocket            │  (Console)       │
│  walled)   │                                      │                  │
│            │  ② control plane sends request        │                  │
│            │ ◀──────────────────────────────────  │  ProxyRequest()  │
│  executes  │      RequestMessage payload          │                  │
│  locally   │                                      │                  │
│            │  ③ agent streams response back        │                  │
│            │ ──────────────────────────────────▶  │  (chunked)       │
└────────────┘      ResponseMessage payloads         └─────────────────┘
```

The package lives in `core/api/websocket/reverse` and provides two layers:

- **Protocol** (`protocol.go`) — the wire format: message structs, framing helpers, and payload (de)serialization.
- **Session manager** (`manager.go`) — the control-plane state machine: session registration/activation, request multiplexing, timeouts, disconnect handling, and reconnect windows.

## When to Use It

| Situation | Fit |
|-----------|-----|
| Agents in private networks; you need to call their HTTP APIs (stats, logs, management) from a central Console | ✅ Primary use case |
| You want agent management without VPNs, port forwarding, or exposing agent ports | ✅ |
| High-throughput data transfer (log shipping, bulk export) | ❌ Use OTLP/gRPC (`otlp_export`) or queues instead — the reverse channel is for control/management traffic |
| One-shot scripts without a running agent process | ❌ Requires a persistent agent websocket |

## Architecture

### Roles

- **Peer** — the agent-side endpoint. Identified by a `peerID` (typically the instance ID). Holds the outbound websocket connection.
- **Control plane** — the server-side consumer of this package. Owns a `SessionManager`, calls `ProxyRequest` for each logical request, and feeds inbound websocket payloads into the manager via the `Handle*Payload` methods.
- **Session** — one established websocket connection between a peer and the control plane. Identified by a `sessionID` issued by the control plane (`RegisterPendingSession`) and confirmed by the agent's `Hello`.

### Connection Lifecycle

```
Control Plane                                Agent (peer)
     │                                            │
     │ (out-of-band: hands sessionID+peerID       │
     │  to the agent, e.g. via install script     │
     │  or managed config)                        │
     │                                            │
     │ ◀──── websocket connect ────────────────── │ ① agent dials out
     │                                            │
     │ ◀──── HELLO {sessionID, peerID} ────────── │ ② handshake
     │  ActivateSession()                         │
     │      (replaces any stale session           │
     │       for the same peer)                   │
     │                                            │
     │ ──── REQUEST {id, method, path, body} ───▶ │ ③ proxied call
     │      (ProxyRequest blocks)                 │    agent executes
     │                                            │    locally
     │ ◀──── RESPONSE chunks {id, chunk} ──────── │ ④ streamed back
     │ ◀──── RESPONSE {id, done, status} ───────── │    (base64)
     │                                            │
     │ ◀──── (ws close) ────────────────────────── │ disconnect
     │  OnDisconnect():                            │
     │   - drops session state                    │
     │   - fails in-flight requests               │
     │      (ErrDisconnected)                     │
```

## Wire Protocol

All messages travel as websocket text payloads produced by the framing helpers in `protocol.go`. A frame is a command prefix followed by a JSON body.

### Message Types

```go
// HelloMessage — sent by the agent right after connecting.
type HelloMessage struct {
    SessionID string
    PeerID    string
}

// RequestMessage — control plane → agent: one proxied HTTP request.
type RequestMessage struct {
    RequestID   string
    PeerID      string
    Method      string
    Path        string
    Headers     http.Header
    AccessToken string      // extracted from the caller's Bearer header
    // body carried via SetBody/BodyBytes (base64)
}

// ResponseMessage — agent → control plane: chunked response stream.
type ResponseMessage struct {
    RequestID string
    PeerID    string
    Chunk     string        // base64-encoded body chunk
    Done      bool          // final frame
    Status    int           // HTTP status (on Done; 0 is not valid —
                            // a Done frame without Status is only legal with Error)
    Error     string        // execution failure (on Done): the agent could not
                            // produce an HTTP response at all; mutually exclusive
                            // with Status
}
```

### Framing Helpers

| Function | Direction | Purpose |
|----------|-----------|---------|
| `FormatHelloCommand(msg)` | agent → CP | Serialize a HELLO |
| `FormatRequestCommand(msg)` | CP → agent | Serialize a REQUEST |
| `FormatResponseCommand(msg)` | agent → CP | Serialize a RESPONSE |
| `ParseHelloPayload(s)` | CP | Parse a HELLO body |
| `ParseRequestPayload(s)` | agent | Parse a REQUEST body |
| `ParseResponsePayload(s)` | CP | Parse a RESPONSE body |
| `msg.SetBody(b)` / `msg.BodyBytes()` | both | Body encode/decode |
| `msg.NormalizedHeaders()` | agent | Canonical header handling |
| `WriteChunkedResponse(write, id, peer, status, body, chunkBytes)` | agent → CP | Stream a response back; rejects non-HTTP status codes (<100 or >599) |
| `WriteFailureResponse(write, id, peer, err)` | agent → CP | Terminate the stream when the request could not be executed at all (no HTTP status exists) |

### Constants

```go
HelloCommand     // HELLO frame prefix
RequestCommand   // REQUEST frame prefix
ResponseCommand  // RESPONSE frame prefix
HeaderPeerID     // header key carrying the peer ID on the dial-out request
```

## Session Manager API

The control plane creates one manager and shares it across connections:

```go
manager := reverse.NewSessionManager(reverse.ManagerOptions{
    DefaultTimeout:   30 * time.Second,   // per logical request
    MaxResponseBytes: 8 * 1024 * 1024,    // 8 MiB response cap
    ReconnectWait:    6 * time.Second,    // retry window after disconnect
    ReconnectPoll:    200 * time.Millisecond,
})
```

### Session Bookkeeping

| Method | Called when | Behavior |
|--------|-------------|----------|
| `RegisterPendingSession(sessionID, peerID)` | before handing credentials to a new agent | Records the expected (sessionID, peerID) pair for the handshake check |
| `HandleHelloPayload(payload)` | HELLO frame arrives | Validates the pair matches a pending session, activates it, and replaces any previous session held by the same peer |
| `OnDisconnect(sessionID)` | websocket closes | Drops pending+active state for that session and fails all its in-flight requests with `ErrDisconnected` |
| `IsConnected(peerID)` | health checks | True when the peer holds an active session |

### Proxying Requests

```go
res, err := manager.ProxyRequest(
    peerID,                    // target agent
    &util.Request{             // logical request
        Method: http.MethodGet,
        Path:   "/_node/stats",
        Context: ctx,
    },
    headers,                   // headers to forward
    send,                      // func(sessionID, payload string) error
    nil,                       // optional: unmarshal JSON body into this
)
```

`ProxyRequest` is the core call:

1. Resolves the peer's active session; `ErrNotConnected` if none.
2. Builds a `RequestMessage` (the caller's `Authorization: Bearer ...` header is lifted into `AccessToken`), registers a pending response, and pushes the frame via the `send` callback (your websocket write path).
3. Blocks until the response completes, the context/deadline expires, or the connection drops.
4. **Reconnect retry**: on a recoverable error (`ErrDisconnected`/`ErrNotConnected`) it waits up to `ReconnectWait` (polling every `ReconnectPoll`) for the agent to re-establish its session, then retries **once**. This makes brief agent restarts transparent to callers.
5. Returns `*util.Result{Body, StatusCode}`; a non-200 status yields both the result and an error.

While a request is in flight, the agent streams `ResponseMessage` chunks; feed each one into the manager from your websocket read loop:

```go
manager.HandleResponsePayload(frameBody) // assembles chunks, enforces the cap
```

### Error Values

| Error | Meaning | Recoverable |
|-------|---------|-------------|
| `ErrDisconnected` | Peer's connection dropped while a request was in flight | ✅ triggers the reconnect-retry path |
| `ErrNotConnected` | No active session for the peer | ✅ same |
| handshake mismatch | HELLO did not match a pending (sessionID, peerID) | ❌ configuration error |
| response exceeds `MaxResponseBytes` | Agent streamed more than the cap | ❌ request fails |
| agent execution failure | Done frame carries `Error` — the agent never produced an HTTP response | ❌ request fails with the agent's error |
| Done frame with neither `Status` nor `Error` | Agent-side protocol violation | ❌ request fails loudly (never silently treated as 200) |

A `Done` frame only means "the response stream ends here" — success or failure
is expressed exclusively by `Status` (a real HTTP status from the locally
executed request) or `Error` (no HTTP response exists). The manager rejects a
bare `Done` with neither field rather than defaulting to 200, so agent-side
bugs surface immediately instead of masquerading as successful empty
responses. Agent implementations should always terminate via
`WriteChunkedResponse` (which validates the status) or `WriteFailureResponse`.

## Security Model

The reverse channel is a **transport**, not an authenticator. Compose it with the framework's existing security layers:

- **Control plane → agent authorization**: the agent's own API authentication applies to the proxied request. The `AccessToken` field carries the caller's bearer token end-to-end, so the agent's API layer can validate it exactly as if the call were direct.
- **Agent → control plane**: the outbound websocket rides the agent's existing authenticated channel to the control plane (e.g. Console's managed websocket with instance credentials).
- **Session binding**: the handshake check (`RegisterPendingSession` → `HandleHelloPayload`) prevents an agent from claiming another peer's session slot.

Do not use the channel to move secrets in bulk — `AccessToken` is the only credential field with dedicated handling; everything else is treated as opaque request data.

## Integration Recipe (Control Plane)

```go
// 1. One manager per process.
reverseManager := reverse.NewSessionManager(reverse.DefaultManagerOptions())

// 2. Issue session credentials when provisioning an agent.
sessionID := util.GetUUID()
reverseManager.RegisterPendingSession(sessionID, agentInstanceID)
// hand {sessionID, peerID: agentInstanceID} to the agent
// (install script, managed config, etc.)

// 3. Wire your websocket server: on each connection, read frames and
//    dispatch by prefix.
api.HandleWebSocketCommand(reverse.HelloCommand, "agent hello", func(...) {
    _ = reverseManager.HandleHelloPayload(body)
})
api.HandleWebSocketCommand(reverse.ResponseCommand, "agent response", func(...) {
    _ = reverseManager.HandleResponsePayload(body)
})
// track the connection's sessionID to call OnDisconnect on close.

// 4. Call agents.
res, err := reverseManager.ProxyRequest(
    agentInstanceID, req, headers,
    func(sessionID, payload string) error {
        return websocketHub.SendPrivateMessage(sessionID, payload)
    }, nil,
)
if reverse.IsRecoverableError(err) {
    // optionally surface "agent temporarily unreachable" to the UI
}
```

## Tuning

| Option | Default | Guidance |
|--------|---------|----------|
| `DefaultTimeout` | 30s | Per logical request. Requests inheriting a context deadline keep it. |
| `MaxResponseBytes` | 8 MiB | Hard cap on assembled responses; raise only for controlled payloads (e.g. log exports). |
| `ReconnectWait` | 6s | How long ProxyRequest tolerates a dropped agent before giving up. Cover agent restarts, not maintenance windows. |
| `ReconnectPoll` | 200ms | Poll frequency inside the reconnect wait. |

## Relationship to Other Framework Facilities

- **Managed config sync** (`modules/configs`) — separate channel; agents pull configs and heartbeat over HTTP. The reverse channel complements it for on-demand RPC.
- **OTLP transport** (`otlp_export` / gateway intake) — the data path for log shipping. The reverse channel is the control path.
- **Console's agent management** — the reference consumer: stats fetching, log tailing, and UI websocket proxying all ride `ProxyRequest` / the session manager.
