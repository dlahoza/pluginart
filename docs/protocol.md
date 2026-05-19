# pluginart Wire Protocol

This document is the normative specification for the pluginart binary wire protocol — the communication layer between a host runtime and a plugin process over a Unix socket or TCP connection. Go, Python, and TypeScript runtimes implement the same framing, handshake, call, and health-check behavior.

---

## Overview

Every host-plugin connection is a single, full-duplex stream (Unix socket or TCP). After an initial handshake, the host serialises calls one at a time over that stream. Messages are wrapped in a fixed-format binary frame. Handshake, health-check and error messages are FlatBuffers-encoded; call payloads are opaque bytes defined by the user's own `.fbs` schema.

---

## Framing

Every message — in both directions — is wrapped in a 9-byte header followed by a variable-length payload.

```
 0       1       2       3       4       5       6       7       8
+-------+-------+-------+-------+-------+-------+-------+-------+-------+
| 0x50  | 0x4C  | 0x47  | 0x4E  |         length (uint32 LE)    | flags |
+-------+-------+-------+-------+-------+-------+-------+-------+-------+
| <payload — `length` bytes> ...
```

| Field    | Size    | Description |
|----------|---------|-------------|
| `magic`  | 4 bytes | ASCII `PLGN` (`0x50 0x4C 0x47 0x4E`). Identifies the protocol. |
| `length` | 4 bytes | Payload size in bytes, unsigned, little-endian. Does not include the 9-byte header. |
| `flags`  | 1 byte  | Message type (see table below). |
| payload  | N bytes | Message body; `N = length`. |

Constraints:

- Maximum payload size: **4 MiB** (4,194,304 bytes). All runtimes must reject frames exceeding this limit and close or fail the connection.
- An empty payload (`length = 0`) is valid for messages that carry no data.

---

## Message types

The `flags` byte identifies the message type. All other bits are reserved and must be zero.

| Name                | Value  | Direction      | Description |
|---------------------|--------|----------------|-------------|
| `HandshakeRequest`  | `0x01` | host → plugin  | First message after connection. Contains contract hash and plugin name. |
| `HandshakeResponse` | `0x02` | plugin → host  | Reply to the handshake. Indicates accept or reject. |
| `CallRequest`       | `0x03` | host → plugin  | Method invocation. Payload is the user's FlatBuffers `CallRequest`. |
| `CallResponse`      | `0x04` | plugin → host  | Successful result. Payload is the user's FlatBuffers `CallResponse`. |
| `PluginError`       | `0x05` | plugin → host  | Structured error in response to a call. |
| `Cancel`            | `0x06` | host → plugin  | Best-effort cancellation of the in-flight call. No-op in v0.1. |
| `Ping`              | `0x07` | host → plugin  | Health check probe. |
| `Pong`              | `0x08` | plugin → host  | Health check reply. |

---

## FlatBuffers schema

Handshake and health-check messages use the protocol-internal FlatBuffers schema (`pluginart.schema.fbs`):

```fbs
namespace pluginart;

table HandshakeRequest {
  contract_hash:    string (required);
  plugin_name:      string (required);
  protocol_version: uint16 = 1;
}

table HandshakeResponse {
  ok:    bool;
  error: string;
}

table Ping { seq: uint64; }
table Pong { seq: uint64; }

table PluginError {
  code:    uint16;
  message: string (required);
  retry:   bool;
}
```

Call payloads (`CallRequest` / `CallResponse`) are defined in the user's own `.fbs` schema. The protocol layer passes them through as opaque byte slices; the generated typed client handles serialisation.

---

## Handshake

The handshake is the first exchange on every new connection. No calls may be made before the handshake completes successfully.

```
host                                plugin
 |                                    |
 |--- HandshakeRequest -------------->|  contract_hash, plugin_name, protocol_version=1
 |                                    |  (plugin checks hash against compiled-in constant)
 |<-- HandshakeResponse --------------|  ok=true            (hash matched)
 |                                    |
 |   OR                               |
 |<-- HandshakeResponse --------------|  ok=false, error="contract hash mismatch"
 |                    [both sides close the connection]
```

Sequence:

1. Host connects to the plugin's Unix socket or TCP address.
2. Host sends a `HandshakeRequest` frame with:
   - `contract_hash` — SHA-256 of the schema file (see [Contract hash](#contract-hash)).
   - `plugin_name` — human-readable name from `pluginart.toml`.
   - `protocol_version` — always `1` in v0.1.
3. Plugin reads the frame, decodes the FlatBuffers table, and compares `contract_hash` against the constant baked in at code-generation time.
4. Plugin sends `HandshakeResponse`:
   - `ok = true` on match; the connection is now live.
   - `ok = false` and a human-readable `error` on mismatch; both sides close the connection immediately.

If the first received frame is not a `HandshakeRequest`, the plugin closes the connection without reply.

---

## Contract hash

The contract hash is the mechanism that guarantees host and plugin were compiled from the same schema.

Format: `sha256:<lowercase-hex-encoded-sha256-of-schema-file-bytes>`

Example: `sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`

- Computed from the raw bytes of the `.fbs` schema file (not its parsed content).
- The CLI embeds the hash into `contract.go` in both the generated host client and the generated plugin stub, as the `ContractHash` constant.
- The host reads the constant from `contract.go` and sends it in `HandshakeRequest.contract_hash`.
- The plugin reads its own `ContractHash` constant and compares.

A schema change requires regenerating both sides. Any mismatch is fatal — the connection is rejected immediately.

---

## Health check

After a successful handshake, the host starts a background ticker that sends a `Ping` at configurable intervals (default: 2 seconds). The plugin must reply with a matching `Pong`.

```
host                        plugin
 |                            |
 |--- Ping (seq=N) ---------->|
 |<-- Pong (seq=N) -----------|
```

- `seq` is a monotonically increasing `uint64` counter, global across all connections on the host.
- The plugin echoes back the same `seq` value.
- The host waits up to 2 seconds for the `Pong`. On timeout or error, the health check is considered failed.
- On repeated failures, the host triggers a plugin restart (binary/Docker plugins only; remote plugins are not restarted).
- Health-check frames may arrive while a call is in flight; the plugin must handle `Ping` at any point after the handshake.

Restart behaviour (binary/Docker):

- After the first failure the host waits 1 second before restarting.
- Each subsequent failure doubles the backoff, capped at 30 seconds.
- After `max_restarts` (default 5) consecutive failures the host stops attempting to restart and logs an error.

---

## Call lifecycle

In v0.1, calls are strictly serialised: the host holds a mutex for the duration of each round-trip, so only one outstanding call exists per connection at any time.

```
host                                plugin
 |                                    |
 |--- CallRequest ------------------->|  raw FlatBuffers payload (user schema)
 |                                    |  plugin.Handle(ctx, payload)
 |<-- CallResponse ------------------|  raw FlatBuffers payload (user schema)
 |                                    |
 |   OR on error:                     |
 |<-- PluginError -------------------|  code, message, retry
```

`PluginError` fields:

| Field     | Type     | Description |
|-----------|----------|-------------|
| `code`    | `uint16` | Application-defined error code. |
| `message` | `string` | Human-readable error description. |
| `retry`   | `bool`   | Hint to the caller that the call is safe to retry. |

The host decodes a `PluginError` into a Go `error` value (`"plugin error <code>: <message>"`). The `retry` field is preserved in the FlatBuffers payload but not yet acted on by the runtime in v0.1.

`Cancel` (`0x06`) is sent by the host when the call's context is cancelled. The plugin receives it but takes no action in v0.1; cancellation is cooperative and future versions may use it to abort in-progress work.

---

## Plugin startup

### Binary and Docker plugins

The plugin process must:

1. Bind its Unix socket (if `PLUGIN_SOCKET` is set) or TCP port (if `PLUGIN_ADDR` is set).
2. Print exactly `READY` followed by a newline to stdout, then flush.
3. Begin accepting connections.

The host scans the plugin's stdout line by line. The first line equal to `READY` (after trimming whitespace) signals that the plugin is ready to accept a connection. Subsequent stdout output is ignored by the host.

If the plugin exits before printing `READY`, or if the startup timeout (default 5 seconds) expires first, the host kills the process and reports an error.

Environment variables injected by the host:

| Variable        | Transport | Value |
|-----------------|-----------|-------|
| `PLUGIN_SOCKET` | Unix      | Absolute path to the Unix socket, e.g. `/tmp/pluginart/<uuid>.sock` |
| `PLUGIN_ADDR`   | TCP       | `host:port`, e.g. `127.0.0.1:54321` |

User-defined environment variables from the `env:` block in `pluginart.toml` are appended to the plugin's environment without modification.

### Remote plugins

The host dials the `address` field from `pluginart.toml` directly over TCP. There is no `READY` signal and no process management. The host performs the handshake immediately on connection. On connection loss, the host reconnects with the same exponential backoff used for binary plugins, but `max_restarts` does not apply.

---

## Transport

| Transport   | Plugin type       | Env var         | Notes |
|-------------|-------------------|-----------------|-------|
| Unix socket | binary, docker    | `PLUGIN_SOCKET` | Default for binary plugins on Linux/macOS. Opt-in for Docker via `transport: unix`. |
| TCP         | binary, docker, remote | `PLUGIN_ADDR` | Default for Docker (cross-platform). Required for remote plugins. |

Both transports expose an identical `net.Conn` to the framing layer; the protocol is identical over both.

---

## Versioning and evolution

The `protocol_version` field in `HandshakeRequest` is reserved for future negotiation. In v0.1, only version `1` is defined. Plugins may reject connections with an unrecognised `protocol_version` by responding with `ok = false`.

Backwards-incompatible protocol changes will increment `protocol_version`. Additive changes (new message types, new optional FlatBuffers fields) may be introduced without a version bump, provided they are handled gracefully by both sides.

The `flags` byte currently carries only the message type. Remaining bits are reserved for future use (e.g. per-frame compression or stream multiplexing) and must be ignored by v0.1 implementations.

The `ShmHandle` shared-memory fast path is reserved in the schema but not implemented until v0.6. TLS for remote-mode connections is deferred to v0.5.
