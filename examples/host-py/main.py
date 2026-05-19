from __future__ import annotations

import os
import sys
from pathlib import Path

import flatbuffers

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "runtimes/python"))

from gen.echo_client import echoClient
from gen.fb.echo import CallRequest, CallResponse, EchoRequest, EchoResponse, RequestPayload, ResponsePayload
from pluginart import PluginManager


def build_echo_call_request(input_text: str) -> bytes:
    b = flatbuffers.Builder(256)
    input_off = b.CreateString(input_text)

    EchoRequest.EchoRequestStart(b)
    EchoRequest.EchoRequestAddInput(b, input_off)
    echo_req = EchoRequest.EchoRequestEnd(b)

    CallRequest.CallRequestStart(b)
    CallRequest.CallRequestAddRequestId(b, 1)
    CallRequest.CallRequestAddPayloadType(b, RequestPayload.RequestPayload.EchoRequest)
    CallRequest.CallRequestAddPayload(b, echo_req)
    req = CallRequest.CallRequestEnd(b)
    b.Finish(req)
    return bytes(b.Output())


def decode_echo_output(resp_bytes: bytes) -> str:
    resp = CallResponse.CallResponse.GetRootAs(bytearray(resp_bytes), 0)
    union_obj = resp.Payload()
    echo_resp = EchoResponse.EchoResponse()
    echo_resp.Init(union_obj.Bytes, union_obj.Pos)
    return (echo_resp.Output() or b"").decode("utf-8")


def main() -> None:
    os.chdir(Path(__file__).resolve().parent)
    req = build_echo_call_request("hello from python host")
    with PluginManager.from_config("pluginart.toml") as manager:
        go_client = echoClient(manager, "echo")
        py_client = echoClient(manager, "echo-py")
        print(f"echo (go):     {decode_echo_output(go_client.Echo(req))}")
        print(f"echo (python): {decode_echo_output(py_client.Echo(req))}")


if __name__ == "__main__":
    main()
