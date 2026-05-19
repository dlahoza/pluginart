from __future__ import annotations

import os
import sys
from pathlib import Path

import flatbuffers

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "runtimes/python"))

from pluginart import PluginManager
from plugins.echo.echo_client import echoClient
from plugins.echo.echo import EchoRequest


def build_echo_payload(input_text: str):
    b = flatbuffers.Builder(256)
    input_off = b.CreateString(input_text)

    EchoRequest.EchoRequestStart(b)
    EchoRequest.EchoRequestAddInput(b, input_off)
    echo_req = EchoRequest.EchoRequestEnd(b)
    return b, echo_req


def main() -> None:
    os.chdir(Path(__file__).resolve().parent)
    with PluginManager.from_config("pluginart.toml") as manager:
        go_client = echoClient(manager, "echo")
        py_client = echoClient(manager, "echo-py")
        ts_client = echoClient(manager, "echo-ts")
        builder, payload = build_echo_payload("hello from python host")
        print(f"echo (go):     {(go_client.Echo(builder, payload).Output() or b'').decode('utf-8')}")
        builder, payload = build_echo_payload("hello from python host")
        print(f"echo (python): {(py_client.Echo(builder, payload).Output() or b'').decode('utf-8')}")
        builder, payload = build_echo_payload("hello from python host")
        print(f"echo (ts):     {(ts_client.Echo(builder, payload).Output() or b'').decode('utf-8')}")


if __name__ == "__main__":
    main()
