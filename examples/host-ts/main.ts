import * as path from 'path';
import * as flatbuffers from 'flatbuffers';
import { PluginManager } from 'pluginart';
import { CallRequest } from './echo/call-request';
import { CallResponse } from './echo/call-response';
import { EchoRequest } from './echo/echo-request';
import { EchoResponse } from './echo/echo-response';
import { echoClient } from './echo_client';
import { RequestPayload } from './echo/request-payload';

function buildEchoCallRequest(input: string): Buffer {
  const b = new flatbuffers.Builder(256);
  const inputOff = b.createString(input);

  EchoRequest.startEchoRequest(b);
  EchoRequest.addInput(b, inputOff);
  const echoReqOff = EchoRequest.endEchoRequest(b);

  CallRequest.startCallRequest(b);
  CallRequest.addRequestId(b, BigInt(1));
  CallRequest.addPayloadType(b, RequestPayload.EchoRequest);
  CallRequest.addPayload(b, echoReqOff);
  const reqOff = CallRequest.endCallRequest(b);
  CallRequest.finishCallRequestBuffer(b, reqOff);

  return Buffer.from(b.asUint8Array());
}

function decodeEchoOutput(respBytes: Buffer): string {
  const bb = new flatbuffers.ByteBuffer(new Uint8Array(respBytes));
  const resp = CallResponse.getRootAsCallResponse(bb);
  const echoResp = resp.payload(new EchoResponse()) as EchoResponse;
  return echoResp.output() ?? '';
}

async function main(): Promise<void> {
  process.chdir(path.resolve(__dirname, '..'));
  const requestBytes = buildEchoCallRequest('hello from ts host');
  const manager = await PluginManager.fromConfig('pluginart.toml');
  try {
    await manager.start();
    const goClient = new echoClient(manager, 'echo');
    const pyClient = new echoClient(manager, 'echo-py');
    console.log(`echo (go):     ${decodeEchoOutput(await goClient.Echo(requestBytes))}`);
    console.log(`echo (python): ${decodeEchoOutput(await pyClient.Echo(requestBytes))}`);
  } finally {
    await manager.shutdown();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
