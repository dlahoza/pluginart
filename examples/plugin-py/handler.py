import flatbuffers
from fb.echo import CallRequest, CallResponse, EchoRequest, EchoResponse, ResponsePayload


def handle(payload: bytes) -> bytes:
    buf = bytearray(payload)
    req = CallRequest.CallRequest.GetRootAs(buf, 0)
    union_obj = req.Payload()
    echo_req = EchoRequest.EchoRequest()
    echo_req.Init(union_obj.Bytes, union_obj.Pos)
    output = (echo_req.Input() or b'').decode('utf-8').upper()

    b = flatbuffers.Builder(128)
    out_off = b.CreateString(output)
    EchoResponse.EchoResponseStart(b)
    EchoResponse.EchoResponseAddOutput(b, out_off)
    echo_resp_off = EchoResponse.EchoResponseEnd(b)
    CallResponse.CallResponseStart(b)
    CallResponse.CallResponseAddRequestId(b, req.RequestId())
    CallResponse.CallResponseAddPayloadType(b, ResponsePayload.ResponsePayload.EchoResponse)
    CallResponse.CallResponseAddPayload(b, echo_resp_off)
    resp_off = CallResponse.CallResponseEnd(b)
    b.Finish(resp_off)
    return bytes(b.Output())
