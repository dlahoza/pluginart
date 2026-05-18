#!/usr/bin/env python3
"""Echo plugin: uppercases the raw call payload. Mirrors examples/plugin-go."""
from pluginart_wire import serve


def handle(payload: bytes) -> bytes:
    return payload.upper()


if __name__ == '__main__':
    serve(handle)
