# pluginart TypeScript runtime

`pluginart` manages plugin lifecycle from `pluginart.toml`, speaks the pluginart wire protocol, and provides plugin-side server helpers.

```ts
import { PluginManager } from 'pluginart';

const manager = await PluginManager.fromConfig('pluginart.toml');
await manager.start();
const response = await manager.call('echo', requestBytes);
await manager.shutdown();
```

Plugin authors can expose a raw-byte handler:

```ts
import { serve } from 'pluginart';

serve((payload) => payload, { contractHash: 'sha256:...' });
```
