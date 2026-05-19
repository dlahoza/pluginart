import flatbuffers
from fb.echo import EchoResponse
from pluginart_helpers import BuildEchoCallResponse, DecodeEchoRequest


def handle(payload: bytes) -> bytes:
    echo_req, call = DecodeEchoRequest(payload)
    output = (echo_req.Input() or b'').decode('utf-8').upper()

    b = flatbuffers.Builder(128)
    out_off = b.CreateString(output)
    EchoResponse.EchoResponseStart(b)
    EchoResponse.EchoResponseAddOutput(b, out_off)
    echo_resp_off = EchoResponse.EchoResponseEnd(b)
    return BuildEchoCallResponse(call, b, echo_resp_off)
