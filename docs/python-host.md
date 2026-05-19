# Python Host Guide

Generate a client:

```bash
pluginart gen client --lang python --schema examples/schema/echo.fbs --out examples/host-py/plugins/echo
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

The repository example in `examples/host-py` keeps generated host code under `plugins/echo` and calls both `examples/plugin-go/plugin-go` and `examples/plugin-py/plugin/plugin.py`.
