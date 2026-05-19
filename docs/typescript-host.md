# TypeScript Host Guide

Generate a client:

```bash
pluginart gen bindings --target host --lang typescript --schema examples/schema/echo.fbs --out examples/host-ts/plugins/echo
pluginart gen plugin --lang typescript --name echo --schema examples/schema/echo.fbs --out examples/plugin-ts
npm install pluginart flatbuffers
```

Node 22 or later is required.

Use the runtime:

```ts
import { PluginManager } from 'pluginart';
import { echoClient } from './plugins/echo/echo_client';

const manager = await PluginManager.fromConfig('pluginart.toml');
try {
  await manager.start();
  const client = new echoClient(manager, 'echo');
  const response = await client.Echo(builder, echoRequestOffset);
} finally {
  await manager.shutdown();
}
```

`builder` and `echoRequestOffset` are the FlatBuffers builder and table offset for the method payload, for example `EchoRequest`. Generated helpers wrap that payload in `CallRequest` and unwrap the returned `CallResponse`.

The repository example in `examples/host-ts` keeps generated host code under `plugins/echo` and `plugins/repeat`, calls Go, Python, and TypeScript plugins, and keeps FlatBuffers payload construction in application code. The `echo` plugins run as binaries; the `repeat` plugins run as Docker containers.

Build the Docker-mode repeat plugins from the repository root before running the full example:

```bash
docker build -f examples/plugin-repeat-go/Dockerfile -t pluginart-repeat-go:local .
docker build -f examples/plugin-repeat-py/Dockerfile -t pluginart-repeat-py:local .
docker build -f examples/plugin-repeat-ts/Dockerfile -t pluginart-repeat-ts:local .
```
