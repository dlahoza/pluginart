# pluginart Python runtime

`pluginart` manages plugin lifecycle from `pluginart.toml`, speaks the pluginart wire protocol, and provides plugin-side server helpers.

```python
from pluginart import PluginManager

with PluginManager.from_config("pluginart.toml") as manager:
    response = manager.call("echo", request_bytes)
```

Plugin authors can expose a raw-byte handler:

```python
from pluginart import serve

def handle(payload: bytes) -> bytes:
    ...

serve(handle, contract_hash="sha256:...")
```
