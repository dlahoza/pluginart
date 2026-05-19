# pluginart

`pluginart` is a CLI plus Go, Python, and TypeScript runtimes for building language-agnostic plugin systems. Hosts manage plugins from `pluginart.toml`; plugins run as binaries, Docker containers, or remote TCP services; all calls use a FlatBuffers-based wire protocol with contract-hash verification at handshake time.

## Install

```bash
go install github.com/dlahoza/pluginart/cmd/pluginart@latest
brew install flatbuffers
```

Runtime packages are published as:

- Go: `github.com/dlahoza/pluginart/pkg/runtime`
- PyPI: `pluginart`
- npm: `pluginart`

## Mental Model

The schema defines payload tables and method names. Generated clients give host code method wrappers. The host runtime starts, health-checks, restarts, calls, and shuts down plugins. Go, Python, and TypeScript generation hide the pluginart RPC envelope while still using FlatBuffers payload builders.

## Quickstart: Go Host

```bash
pluginart gen bindings --target host --lang go --schema examples/schema/echo.fbs --out examples/host-go/plugins
pluginart gen bindings --target plugin --lang go --schema examples/schema/echo.fbs --out examples/plugin-go/plugin
cd examples/plugin-go && go build -o plugin-go .
cd ../host-go && go run .
```

Go hosts use `runtime.NewManagerFromConfig("pluginart.toml")` and generated clients that wrap `manager.Call`.

## Quickstart: Python Host

```bash
pluginart gen bindings --target host --lang python --schema examples/schema/echo.fbs --out examples/host-py/plugins/echo
pluginart gen plugin --lang python --name echo --schema examples/schema/echo.fbs --out examples/plugin-py
pip install pluginart flatbuffers
python examples/host-py/main.py
```

Python hosts use:

```python
from pluginart import PluginManager
from plugins.echo.echo_client import echoClient

with PluginManager.from_config("pluginart.toml") as manager:
    client = echoClient(manager, "echo")
    response = client.Echo(builder, echo_request_offset)
```

## Quickstart: TypeScript Host

```bash
pluginart gen bindings --target host --lang typescript --schema examples/schema/echo.fbs --out examples/host-ts/plugins/echo
npm install pluginart flatbuffers
npm run build && npm start
```

TypeScript hosts use:

```ts
import { PluginManager } from 'pluginart';

const manager = await PluginManager.fromConfig('pluginart.toml');
await manager.start();
const client = new EchoClient(manager, 'echo');
const response = await client.Echo(builder, echoRequestOffset);
await manager.shutdown();
```

## Plugin Modes

| `type` | Host behavior | Transport default |
| --- | --- | --- |
| `binary` | Execs `path`, injects `PLUGIN_SOCKET` or `PLUGIN_ADDR`, waits for `READY` | Unix socket |
| `docker` | Runs `docker run`, injects `PLUGIN_ADDR`, waits for `READY` in logs | TCP |
| `remote` | Dials `address` directly | TCP |

Config-driven lifecycle is available in Go, Python, and TypeScript runtimes.

## Docs

- [Getting started](docs/getting-started.md)
- [CLI reference](docs/cli.md)
- [Config reference](docs/config.md)
- [Schema guide](docs/schema.md)
- [Python host guide](docs/python-host.md)
- [TypeScript host guide](docs/typescript-host.md)
- [Wire protocol](docs/protocol.md)
- [Releasing](docs/releasing.md)

## Status

`v0.2.0` provides Go, Python, and TypeScript host/plugin runtimes, binary/remote/docker modes, Unix/TCP transports, FlatBuffers handshake, contract-hash verification, Ping/Pong health checks, restart backoff, schema initialization, validation, and client/plugin generation.

Per-call cancellation, observability, compression, TLS for remote mode, and shared-memory fast paths are deferred.

## License

[MIT](LICENSE)
