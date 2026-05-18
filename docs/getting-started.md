# Getting started with pluginart

This guide walks through the full workflow: define a plugin API, generate code, implement the plugin, and run it - in under ten minutes.

## Prerequisites

- Go 1.23 or later
- `flatc` - the FlatBuffers compiler

Install `flatc` on macOS:

```bash
brew install flatbuffers
```

On other platforms, download a release binary from https://github.com/google/flatbuffers/releases.

Verify it is on your PATH:

```bash
flatc --version
```

## 1. Install the CLI

```bash
go install github.com/dlahoza/pluginart/cmd/pluginart@latest
```

## 2. Define your plugin API

`pluginart init schema` generates a FlatBuffers schema boilerplate. The schema is the contract between host and plugin - both sides are generated from it, and the SHA-256 hash of the file is verified at connection time.

```bash
pluginart init schema --name transform
```

Output:

```
./schema/
├── transform.fbs
└── README.md
```

Generated `transform.fbs`:

```fbs
namespace transform;

table ExampleRequest {
  input: string;
}

table ExampleResponse {
  output: string;
}

union RequestPayload  { ExampleRequest }
union ResponsePayload { ExampleResponse }

table CallRequest {
  request_id:       uint64;
  deadline_unix_ms: int64;
  trace_id:         string;
  span_id:          string;
  payload:          RequestPayload;
}

table CallResponse {
  request_id: uint64;
  payload:    ResponsePayload;
}

table ShmHandle { offset: uint64; length: uint64; }

table PluginError {
  code:    uint16;
  message: string;
  retry:   bool;
}

root_type CallRequest;
```

To add a method, add a table pair and register both in the unions:

```fbs
table TransformRequest {
  text:      string;
  uppercase: bool;
}

table TransformResponse {
  result: string;
}

union RequestPayload  { ExampleRequest, TransformRequest }
union ResponsePayload { ExampleResponse, TransformResponse }
```

Each entry in `RequestPayload` corresponds to one callable method. The union member name (e.g. `TransformRequest`) is stripped of the `Request` suffix to derive the method name (`Transform`).

## 3. Generate the host-side client

```bash
pluginart gen client --lang go --schema ./schema/transform.fbs
```

Output:

```
./gen/go/transform/
├── transform_client.go      # Typed client: TransformRequest → TransformResponse
├── flatbuffers.go           # Generated FlatBuffers accessors (from flatc)
└── contract.go              # ContractHash constant - verified at every connection
```

The generated client wraps `runtime.PluginManager.Call` with typed request/response structs. Import it in your host application:

```go
import "yourmodule/gen/go/transform"

client := transform.NewClient(manager, "transform")
resp, err := client.Example(ctx, &transform.ExampleRequest{Input: "hello"})
```

## 4. Generate the plugin skeleton

```bash
pluginart gen plugin --lang go --name transform --schema ./schema/transform.fbs
```

Output:

```
./transform-plugin/
├── main.go          # Entry point: listens, writes READY, serves calls
├── plugin.go        # Handler implementation - edit this file
├── flatbuffers.go   # Generated FlatBuffers code
├── contract.go      # ContractHash - must match the host's gen/go/transform/contract.go
├── go.mod
└── Dockerfile
```

`main.go` (generated, do not edit):

```go
package main

import (
    "fmt"
    "net"
    "os"
    "github.com/dlahoza/pluginart/pkg/protocol"
)

func main() {
    var (
        ln  net.Listener
        err error
    )
    switch {
    case os.Getenv("PLUGIN_SOCKET") != "":
        ln, err = net.Listen("unix", os.Getenv("PLUGIN_SOCKET"))
    case os.Getenv("PLUGIN_ADDR") != "":
        ln, err = net.Listen("tcp", os.Getenv("PLUGIN_ADDR"))
    default:
        fmt.Fprintln(os.Stderr, "PLUGIN_SOCKET or PLUGIN_ADDR must be set")
        os.Exit(1)
    }
    if err != nil {
        fmt.Fprintf(os.Stderr, "listen: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("READY")

    server := protocol.NewServer(ln, &PluginHandler{}, ContractHash)
    if err := server.Serve(); err != nil {
        fmt.Fprintf(os.Stderr, "serve: %v\n", err)
        os.Exit(1)
    }
}
```

The host injects exactly one of `PLUGIN_SOCKET` (Unix domain socket path) or `PLUGIN_ADDR` (TCP `host:port`) into the plugin process environment. The plugin listens on whichever is set, then writes `READY` to stdout. The host reads that line before dialling the connection.

## 5. Implement the plugin

Edit `transform-plugin/plugin.go`. The generated stub looks like this:

```go
package main

import (
    "context"
    "fmt"
)

type PluginHandler struct{}

func (h *PluginHandler) Handle(ctx context.Context, payload []byte) ([]byte, error) {
    // TODO: decode payload and return response
    return nil, fmt.Errorf("not implemented")
}
```

`Handle` receives raw FlatBuffers `CallRequest` bytes and must return raw FlatBuffers `CallResponse` bytes. Use the generated helpers in `flatbuffers.go` to decode the request and encode the response. A complete implementation:

```go
func (h *PluginHandler) Handle(ctx context.Context, payload []byte) ([]byte, error) {
    req := GetRootAsCallRequest(payload, 0)
    switch req.PayloadType() {
    case RequestPayloadExampleRequest:
        var table ExampleRequest
        req.Payload(&table)
        result := strings.ToUpper(table.Input())
        return buildExampleResponse(result), nil
    default:
        return nil, fmt.Errorf("unknown method %d", req.PayloadType())
    }
}
```

## 6. Configure the host

Create `pluginart.toml` in your host application's working directory:

```toml
version = 1

[defaults]
startup_timeout  = "5s"
shutdown_timeout = "10s"
health_interval  = "2s"
max_restarts     = 5

[[plugins]]
name = "transform"
type = "binary"
path = "./transform-plugin/transform-plugin"
```

`type = "binary"` means the host execs the binary directly, injects `PLUGIN_SOCKET`, and waits for `READY`. The `path` is relative to the working directory when the host process runs.

For available config fields see the full reference in `docs/config.md`.

## 7. Run the example

Build the plugin binary first:

```bash
cd transform-plugin
go build -o transform-plugin .
cd ..
```

Run the host:

```bash
go run ./cmd/myapp
```

The host starts the plugin binary, waits for `READY`, dials the socket, verifies the contract hash, and is ready to accept calls.

For a fully working runnable version that skips FlatBuffers encoding (raw bytes only, to show the mechanics without the schema toolchain), see:

- `examples/plugin-go/` - a minimal echo plugin
- `examples/host-go/` - the matching host that calls it

Build and run those:

```bash
# build the plugin
cd examples/plugin-go && go build -o plugin-go . && cd ../..

# run the host (it calls the plugin and prints the response)
cd examples/host-go && go run .
# echo response: HELLO FROM HOST
```

## Next steps

**Docker mode** - run a plugin as a container without changing a line in the host. Set `type = "docker"` and `image = "yourimage:tag"` in `pluginart.toml`. The host manages the container lifecycle, reading `READY` from container stdout. TCP is the default transport for Docker (compatible with all platforms including macOS); Unix socket is available on Linux via `transport = "unix"`.

**Remote mode** - connect to a plugin already running on another host. Set `type = "remote"` and `address = "host:port"`. No process lifecycle management applies; the host dials, handshakes, and health-checks over the live connection.

**Multiple languages** - Python and TypeScript plugin generation (`pluginart gen plugin --lang python|typescript`) is planned for v0.2, along with Docker mode and a `pluginart validate` command.
