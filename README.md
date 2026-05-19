# pluginart

A Go library and CLI for building language-agnostic plugin systems. Plugins are separate processes — written in Go, Python, or TypeScript — connected over a Unix socket or TCP, speaking a FlatBuffers-based binary protocol with a typed contract verified at connection time.

The host application imports one package and reads one config file. Plugin authors run one command to get a working skeleton.

## How it works

```
┌─────────────────────────────────────┐      Unix socket / TCP
│  Host application                   │ ◄──────────────────────► Plugin process
│                                     │      FlatBuffers frames
│  runtime.NewManagerFromConfig(...)  │
│  manager.Call(ctx, "transform", …)  │      Any language. Same binary,
│                                     │      Docker image, or remote host.
└─────────────────────────────────────┘
```

The schema (`.fbs` file) is the single source of truth. Its SHA-256 hash is verified at every connection — version skew fails loudly at handshake time, not at call time.

## Install

```bash
# CLI (requires flatc on PATH)
go install github.com/dlahoza/pluginart/cmd/pluginart@latest

# flatc — FlatBuffers compiler
brew install flatbuffers   # macOS
# or: https://github.com/google/flatbuffers/releases
```

## Quickstart

```bash
# 1. Define the API
pluginart init schema --name transform
# Edit ./schema/transform.fbs — add your request/response tables and register them in the unions

# 2. Generate the host-side typed client
pluginart gen client --lang go --schema ./schema/transform.fbs
# → ./gen/go/transform/
# Also supported: --lang python, --lang typescript

# 3. Generate the plugin skeleton
pluginart gen plugin --lang go --name transform --schema ./schema/transform.fbs
# → ./transform-plugin/
# Also supported: --lang python, --lang typescript

# 4. Implement the plugin
# Edit ./transform-plugin/plugin.go — fill in Handle()

# 5. Build the plugin
cd transform-plugin && go build -o transform-plugin .

# 6. Configure the host
cat > pluginart.toml << 'EOF'
version = 1

[[plugins]]
name         = "transform"
type         = "binary"
path         = "./transform-plugin/transform-plugin"
contract_hash = "sha256:<hash from pluginart validate --schema ./schema/transform.fbs>"
EOF

# 7. Use it in Go
```

```go
import (
    "github.com/dlahoza/pluginart/pkg/runtime"
    "yourmodule/gen/go/transform"
)

manager, _ := runtime.NewManagerFromConfig("pluginart.toml")
manager.Start(ctx)
defer manager.Shutdown(context.Background())

client := transform.NewClient(manager, "transform")
resp, err := client.Example(ctx, &transform.ExampleRequest{Input: "hello"})
```

For runnable end-to-end examples (Go host + Go plugin, Go host + Python plugin, TypeScript host), see [examples/](examples/).

## Plugin modes

| `type`    | What the host does                                                  | Transport default |
|-----------|---------------------------------------------------------------------|-------------------|
| `binary`  | Execs the binary, injects `PLUGIN_SOCKET`/`PLUGIN_ADDR`, reads `READY` from stdout | Unix socket       |
| `docker`  | Runs the image via `docker run`, manages the container lifecycle    | TCP               |
| `remote`  | Dials `address` directly — no process management                    | TCP               |

The host application does not know or care which mode is in use.

## Configuration

```toml
# pluginart.toml
version = 1

[defaults]
startup_timeout    = "5s"
shutdown_timeout   = "10s"
health_interval    = "2s"
max_restarts       = 5
restart_backoff_max = "30s"
max_message_bytes  = 4194304

[[plugins]]
name          = "transform"
type          = "binary"
path          = "./plugins/transform"
contract_hash = "sha256:<hex>"
env           = { LOG_LEVEL = "info" }

[[plugins]]
name          = "filter"
type          = "docker"
image         = "ghcr.io/myorg/filter-plugin:v1.2.0"
contract_hash = "sha256:<hex>"
env           = { LOG_LEVEL = "debug" }

[[plugins]]
name          = "scorer"
type          = "remote"
address       = "scorer.internal:9090"
contract_hash = "sha256:<hex>"
```

Plugin configuration is env vars only. The `env` table is injected verbatim into the plugin process environment — no special serialisation.

## Wire protocol

Frames are: `[magic: 4B "PLGN"][length: uint32 LE][flags: 1B][payload: N bytes]`.

Handshake messages use FlatBuffers (defined in `pluginart.schema.fbs`, pre-compiled into `pkg/protocol/fb/`). Call payloads are opaque bytes to the runtime — the generated typed client handles FlatBuffers serialisation of the user's schema. Health checks use application-level Ping/Pong frames over the same connection.

## Status

**v0.2** — current release.

| Feature                                    | Status  |
|--------------------------------------------|---------|
| Binary and remote plugin modes             | ✓       |
| Unix socket and TCP transports             | ✓       |
| FlatBuffers handshake + contract hash      | ✓       |
| Ping/Pong health checks + restart backoff  | ✓       |
| `pluginart init schema`                    | ✓       |
| `pluginart gen client/plugin --lang go`    | ✓       |
| Docker plugin mode                         | ✓       |
| `pluginart gen client/plugin --lang python`     | ✓  |
| `pluginart gen client/plugin --lang typescript` | ✓  |
| `pluginart validate`                       | ✓       |
| Protocol docs (`docs/protocol.md`)         | ✓       |
| Per-call deadline and cancellation         | v0.3    |
| Observability (OTel)                       | v0.4    |
| Compression (LZ4 / ZSTD)                   | v0.5    |
| TLS for remote mode                        | v0.5    |
| Shared memory fast path                    | v0.6    |

## License

[MIT](LICENSE)
