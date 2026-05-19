import * as flatbuffers from 'flatbuffers';
import { EchoResponse } from './plugin/fb/echo/echo-response';
import { BuildEchoCallResponse, DecodeEchoRequest } from './plugin/pluginart_helpers';

export function handle(payload: Buffer): Buffer {
  const request = DecodeEchoRequest(payload);
  const input = request.payload.input() ?? '';
  const output = input.toUpperCase();

  const builder = new flatbuffers.Builder(128);
  const outputOffset = builder.createString(output);
  EchoResponse.startEchoResponse(builder);
  EchoResponse.addOutput(builder, outputOffset);
  const responseOffset = EchoResponse.endEchoResponse(builder);
  return BuildEchoCallResponse(request.call, builder, responseOffset);
}
