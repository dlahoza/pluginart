# Python Host Guide

Generate a client:

```bash
pluginart gen client --lang python --schema examples/schema/echo.fbs --out examples/host-py/gen
pip install pluginart flatbuffers
```

Create `pluginart.toml` beside your host script and point entries at the plugin binaries or remote addresses.

Use the runtime:

```python
from pluginart import PluginManager
from gen.echo_client import echoClient

with PluginManager.from_config("pluginart.toml") as manager:
    client = echoClient(manager, "echo")
    response_bytes = client.Echo(request_bytes)
```

`request_bytes` must be a complete schema `CallRequest` FlatBuffer. Decode `response_bytes` as the schema `CallResponse`.

The repository example in `examples/host-py` calls both `examples/plugin-go/plugin-go` and `examples/plugin-py/plugin.py`.
