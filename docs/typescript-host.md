# TypeScript Host Guide

Generate a client:

```bash
pluginart gen client --lang typescript --schema examples/schema/echo.fbs --out examples/host-ts/gen
npm install pluginart flatbuffers
```

Node 22 or later is required.

Use the runtime:

```ts
import { PluginManager } from 'pluginart';
import { echoClient } from './gen/echo_client';

const manager = await PluginManager.fromConfig('pluginart.toml');
try {
  await manager.start();
  const client = new echoClient(manager, 'echo');
  const responseBytes = await client.Echo(requestBytes);
} finally {
  await manager.shutdown();
}
```

`requestBytes` must be a complete schema `CallRequest` FlatBuffer. Decode `responseBytes` as the schema `CallResponse`.

The repository example in `examples/host-ts` keeps FlatBuffers request/response construction in application code.
