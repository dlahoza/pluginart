# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`pluginart` is a Go library and CLI (`github.com/dlahoza/pluginart`) for building language-agnostic plugin systems. Plugins are separate OS processes connected over Unix socket or TCP, speaking a FlatBuffers-based binary protocol with a typed contract verified at handshake time. The host imports `pkg/runtime`; plugin authors use the `pluginart` CLI to scaffold everything they need.

## Build and test

```bash
go build ./...
go test ./...
go test ./pkg/protocol/... -run TestHandshake -v   # single test
go vet ./...
staticcheck ./...                                   # if installed
```

The CLI binary:
```bash
go build -o pluginart ./cmd/pluginart
```

FlatBuffers code generation (run after editing any `.fbs` file):
```bash
flatc --go --gen-mutable -o pkg/protocol/fb pluginart.schema.fbs
```
`flatc` must be on PATH. Install: `brew install flatbuffers` or https://github.com/google/flatbuffers/releases

## Architecture

```
cmd/pluginart/        CLI — cobra commands: init schema, gen client, gen plugin, validate
pkg/
  transport/          net.Conn factory: UnixTransport, TCPTransport implement Dialer/Listener
  schema/             Parse .fbs files, compute SHA-256 contract hash
  protocol/           Wire framing over any net.Conn + FlatBuffers handshake
  runtime/            PluginManager — lifecycle (binary exec / remote dial), Call dispatch
templates/            Go text/template files embedded via embed.FS, one dir per language
```

### Wire protocol

Frame layout (little-endian):
```
[magic: 4 bytes 0x50 0x4C 0x47 0x4E][length: uint32][flags: uint8][payload: N bytes]
```

`flags` encodes message type: HandshakeRequest, HandshakeResponse, CallRequest, CallResponse, PluginError, Cancel, Ping, Pong.

Handshake messages are FlatBuffers-encoded (from `pluginart.schema.fbs`). Call payloads are opaque byte slices — the runtime passes them through untouched. The generated typed client handles FlatBuffers serialisation of the user's schema.

### Contract hash

SHA-256 of the raw `.fbs` schema file content, formatted as `sha256:<hex>`. The plugin sends this in `HandshakeRequest`; the host rejects the connection if it does not match the hash baked into the generated client (`contract.go`).

### Plugin lifecycle (binary mode)

1. Host execs the binary, injects `PLUGIN_SOCKET` (Unix) or `PLUGIN_ADDR` (TCP) env var.
2. Plugin listens, then writes `READY\n` to stdout.
3. Host reads `READY`, dials the socket/address, performs handshake.
4. On unhealthy or crashed plugin: restart with exponential backoff up to `max_restarts`.

### Plugin lifecycle (remote mode)

Host dials `address` over TCP directly. No process management, no `READY` signal. On connection loss, manager reconnects with exponential backoff. `max_restarts` does not apply.

### Code generation (CLI)

`pluginart gen client --lang go` and `pluginart gen plugin --lang go`:
1. Parse the `.fbs` file with `pkg/schema` to extract namespace, table names, and union members.
2. Invoke `flatc --go` to produce FlatBuffers accessor code.
3. Render pluginart-specific wrapper files from `templates/<lang>/`.
4. Write output to `--out` directory.

CLI fails with a clear error and installation link if `flatc` is not found on PATH.

## Key dependencies

- `github.com/google/flatbuffers` — FlatBuffers Go runtime
- `github.com/spf13/cobra` — CLI
- `github.com/BurntSushi/toml` — config parsing

## Design decisions from PRD

- Plugin configuration is env vars only (`env:` block in `pluginart.toml`). No serialisation, no handshake involvement.
- Docker transport default is TCP (cross-platform). Unix socket is opt-in via `transport: unix` and only works on Linux.
- Schema versioning uses namespace suffix (`namespace myplugin.v2`); plugin declares supported versions in handshake.
- Shared memory fast path is reserved in the schema (`ShmHandle`) but not implemented until v0.6.
- TLS for remote mode is deferred to v0.5.
