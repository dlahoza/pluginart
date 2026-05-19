# TypeScript Host Guide

Generate a client:

```bash
pluginart gen bindings --target host --lang typescript --schema examples/schema/echo.fbs --out examples/host-ts/plugins/echo
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

The repository example in `examples/host-ts` keeps generated host code under `plugins/echo` and keeps FlatBuffers payload construction in application code.
