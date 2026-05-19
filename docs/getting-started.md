# Getting Started

This workflow starts from one `.fbs` schema and ends with a host that can call plugins in Go, Python, or TypeScript.

## Prerequisites

- `flatc` on `PATH`
- Go 1.23 or later for the CLI and Go runtime
- Python 3.11 or later for the Python runtime
- Node 22 or later for the TypeScript runtime

```bash
go install github.com/dlahoza/pluginart/cmd/pluginart@latest
```

## 1. Create A Schema

```bash
pluginart init schema --name echo
```

The schema defines `CallRequest`, `CallResponse`, request/response tables, and `RequestPayload` / `ResponsePayload` unions. Each request union member becomes a generated client method by trimming the `Request` suffix.

## 2. Generate Host Clients

Go:

```bash
pluginart gen client --lang go --schema schema/echo.fbs --out gen/go/echo
```

Python:

```bash
pluginart gen client --lang python --schema schema/echo.fbs --out gen/python
```

TypeScript:

```bash
pluginart gen client --lang typescript --schema schema/echo.fbs --out gen/typescript
```

Go and Python clients include method wrappers plus generated helpers that wrap and unwrap the pluginart `CallRequest` / `CallResponse` envelope. Application code still builds the method payload table with FlatBuffers, then passes the builder and payload offset to the generated client. TypeScript clients expose method names over raw schema `CallRequest` bytes and return raw schema `CallResponse` bytes.

## 3. Generate A Plugin

```bash
pluginart gen plugin --lang go --name echo --schema schema/echo.fbs --out echo-plugin-go
pluginart gen plugin --lang python --name echo --schema schema/echo.fbs --out echo-plugin-py
pluginart gen plugin --lang typescript --name echo --schema schema/echo.fbs --out echo-plugin-ts
```

Generated Python and TypeScript plugin entrypoints import `serve` from the runtime package. Handler files stay focused on decoding `CallRequest` and encoding `CallResponse`.

## 4. Configure The Host

```toml
version = 1

[[plugins]]
name = "echo"
type = "binary"
path = "./echo-plugin-go/echo-plugin"
contract_hash = "sha256:<hash from pluginart validate --schema schema/echo.fbs>"
```

The runtime reads `pluginart.toml`, starts local plugins, performs the handshake, checks health, restarts binary plugins with backoff, and shuts them down.

## 5. Run Examples

```bash
cd examples/plugin-go && go build -o plugin-go .
cd ../host-go && go run .
python ../host-py/main.py
cd ../host-ts && npm install && npm run build && npm start
```

The examples keep FlatBuffers payload construction visible. Go and Python examples use generated helpers for the pluginart RPC envelope; TypeScript still shows the raw envelope bytes.
