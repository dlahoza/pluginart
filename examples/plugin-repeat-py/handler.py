from flatbuffers import Builder

from fb.repeat import RepeatResponse
from pluginart_helpers import BuildRepeatCallResponse, DecodeRepeatRequest

def handle(payload: bytes) -> bytes:
    request, call = DecodeRepeatRequest(payload)
    input_text = (request.Input() or b"").decode("utf-8")
    output = input_text * request.Count()

    builder = Builder(128)
    output_off = builder.CreateString(output)
    RepeatResponse.RepeatResponseStart(builder)
    RepeatResponse.RepeatResponseAddOutput(builder, output_off)
    response = RepeatResponse.RepeatResponseEnd(builder)
    return BuildRepeatCallResponse(call, builder, response)
