import * as path from 'path';
import * as flatbuffers from 'flatbuffers';
import { PluginManager } from 'pluginart';
import { EchoRequest } from './plugins/echo/echo/echo-request';
import { EchoResponse } from './plugins/echo/echo/echo-response';
import { echoClient } from './plugins/echo/echo_client';
import { RepeatRequest } from './plugins/repeat/repeat/repeat-request';
import { RepeatResponse } from './plugins/repeat/repeat/repeat-response';
import { repeatClient } from './plugins/repeat/repeat_client';

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

function buildRepeatPayload(input: string, count: number): { builder: flatbuffers.Builder; payload: flatbuffers.Offset } {
  const b = new flatbuffers.Builder(256);
  const inputOff = b.createString(input);

  RepeatRequest.startRepeatRequest(b);
  RepeatRequest.addInput(b, inputOff);
  RepeatRequest.addCount(b, count);
  const repeatReqOff = RepeatRequest.endRepeatRequest(b);

  return { builder: b, payload: repeatReqOff };
}

function decodeRepeatOutput(response: RepeatResponse): string {
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
    const tsClient = new echoClient(manager, 'echo-ts');
    request = buildEchoPayload('hello from ts host');
    console.log(`echo (ts):     ${decodeEchoOutput(await tsClient.Echo(request.builder, request.payload))}`);

    const repeatGoClient = new repeatClient(manager, 'repeat-go');
    const repeatPyClient = new repeatClient(manager, 'repeat-py');
    const repeatTsClient = new repeatClient(manager, 'repeat-ts');
    let repeatRequest = buildRepeatPayload('ha', 3);
    console.log(`repeat (go):     ${decodeRepeatOutput(await repeatGoClient.Repeat(repeatRequest.builder, repeatRequest.payload))}`);
    repeatRequest = buildRepeatPayload('ha', 3);
    console.log(`repeat (python): ${decodeRepeatOutput(await repeatPyClient.Repeat(repeatRequest.builder, repeatRequest.payload))}`);
    repeatRequest = buildRepeatPayload('ha', 3);
    console.log(`repeat (ts):     ${decodeRepeatOutput(await repeatTsClient.Repeat(repeatRequest.builder, repeatRequest.payload))}`);
  } finally {
    await manager.shutdown();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
