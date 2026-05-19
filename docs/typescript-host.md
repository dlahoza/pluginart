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
  const response = await client.Echo(builder, echoRequestOffset);
} finally {
  await manager.shutdown();
}
```

`builder` and `echoRequestOffset` are the FlatBuffers builder and table offset for the method payload, for example `EchoRequest`. Generated helpers wrap that payload in `CallRequest` and unwrap the returned `CallResponse`.

The repository example in `examples/host-ts` keeps FlatBuffers payload construction in application code.
