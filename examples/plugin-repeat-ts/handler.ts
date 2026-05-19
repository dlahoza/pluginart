// Edit this file to implement your plugin logic.

import * as flatbuffers from 'flatbuffers';
import { RepeatResponse } from './plugin/fb/repeat/repeat-response';
import { BuildRepeatCallResponse, DecodeRepeatRequest } from './plugin/pluginart_helpers';

export function handle(payload: Buffer): Buffer {
  const request = DecodeRepeatRequest(payload);
  const input = request.payload.input() ?? '';
  const output = input.repeat(request.payload.count());

  const builder = new flatbuffers.Builder(128);
  const outputOff = builder.createString(output);
  RepeatResponse.startRepeatResponse(builder);
  RepeatResponse.addOutput(builder, outputOff);
  const response = RepeatResponse.endRepeatResponse(builder);
  return BuildRepeatCallResponse(request.call, builder, response);
}
