import * as path from 'path';
import * as flatbuffers from 'flatbuffers';
import { PluginManager } from 'pluginart';
import { EchoRequest } from './echo/echo-request';
import { EchoResponse } from './echo/echo-response';
import { echoClient } from './echo_client';

function buildEchoPayload(input: string): { builder: flatbuffers.Builder; payload: flatbuffers.Offset } {
  const b = new flatbuffers.Builder(256);
  const inputOff = b.createString(input);

  EchoRequest.startEchoRequest(b);
  EchoRequest.addInput(b, inputOff);
  const echoReqOff = EchoRequest.endEchoRequest(b);

  return { builder: b, payload: echoReqOff };
}

function decodeEchoOutput(response: EchoResponse): string {
  return response.output() ?? '';
}

async function main(): Promise<void> {
  process.chdir(path.resolve(__dirname, '..'));
  const manager = await PluginManager.fromConfig('pluginart.toml');
  try {
    await manager.start();
    const goClient = new echoClient(manager, 'echo');
    const pyClient = new echoClient(manager, 'echo-py');
    let request = buildEchoPayload('hello from ts host');
    console.log(`echo (go):     ${decodeEchoOutput(await goClient.Echo(request.builder, request.payload))}`);
    request = buildEchoPayload('hello from ts host');
    console.log(`echo (python): ${decodeEchoOutput(await pyClient.Echo(request.builder, request.payload))}`);
  } finally {
    await manager.shutdown();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
