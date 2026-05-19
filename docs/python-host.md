# Python Host Guide

Generate a client:

```bash
pluginart gen bindings --target host --lang python --schema examples/schema/echo.fbs --out examples/host-py/plugins/echo
pip install pluginart flatbuffers
```

Create `pluginart.toml` beside your host script and point entries at the plugin binaries or remote addresses.

Use the runtime:

```python
from pluginart import PluginManager
from plugins.echo.echo_client import echoClient

with PluginManager.from_config("pluginart.toml") as manager:
    client = echoClient(manager, "echo")
    response = client.Echo(builder, echo_request_offset)
```

`builder` and `echo_request_offset` are the FlatBuffers builder and table offset for the method payload, for example `EchoRequest`. Generated helpers wrap that payload in `CallRequest` and unwrap the returned `CallResponse`.

The repository example in `examples/host-py` keeps generated host code under `plugins/echo` and `plugins/repeat`. It calls the Go, Python, and TypeScript `echo` plugins as binaries, and calls the Go, Python, and TypeScript `repeat` plugins as Docker containers.

Build the Docker-mode repeat plugins from the repository root before running the full example:

```bash
docker build -f examples/plugin-repeat-go/Dockerfile -t pluginart-repeat-go:local .
docker build -f examples/plugin-repeat-py/Dockerfile -t pluginart-repeat-py:local .
docker build -f examples/plugin-repeat-ts/Dockerfile -t pluginart-repeat-ts:local .
```
