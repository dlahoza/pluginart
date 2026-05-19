import * as path from 'path';
import * as flatbuffers from 'flatbuffers';
import { spawnPlugin, echo, kill } from './plugin_runner';
import { CallRequest } from './echo/echo/call-request';
import { CallResponse } from './echo/echo/call-response';
import { EchoRequest } from './echo/echo/echo-request';
import { EchoResponse } from './echo/echo/echo-response';
import { RequestPayload } from './echo/echo/request-payload';
import { ResponsePayload } from './echo/echo/response-payload';

const DIR = path.resolve(__dirname, '../..');

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
  const goPlugin = await spawnPlugin(
    path.join(DIR, 'plugin-go', 'plugin-go'),
    [],
    'go',
  );

  const pyPlugin = await spawnPlugin(
    'python3',
    [path.join(DIR, 'plugin-py', 'plugin.py')],
    'py',
  );

  const requestBytes = buildEchoCallRequest('hello from ts host');

  const goRespBytes = await echo(goPlugin, requestBytes);
  const goOutput = decodeEchoOutput(goRespBytes);

  const pyRespBytes = await echo(pyPlugin, requestBytes);
  const pyOutput = decodeEchoOutput(pyRespBytes);

  console.log(`echo (go):     ${goOutput}`);
  console.log(`echo (python): ${pyOutput}`);

  kill(goPlugin);
  kill(pyPlugin);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
