from __future__ import annotations

import os
import socket
import sys
import threading
from collections.abc import Callable

from .protocol import (
    MSG_CALL_REQUEST,
    MSG_CALL_RESPONSE,
    MSG_CANCEL,
    MSG_HANDSHAKE_REQUEST,
    MSG_HANDSHAKE_RESPONSE,
    MSG_PING,
    MSG_PLUGIN_ERROR,
    MSG_PONG,
    build_handshake_response,
    build_plugin_error,
    build_pong,
    parse_contract_hash,
    parse_ping_seq,
    recv_frame,
    send_frame,
)


def _handle_conn(conn: socket.socket, handler: Callable[[bytes], bytes], contract_hash: str) -> None:
    try:
        msg_type, payload = recv_frame(conn)
        if msg_type != MSG_HANDSHAKE_REQUEST:
            return
        if parse_contract_hash(payload) != contract_hash:
            send_frame(conn, MSG_HANDSHAKE_RESPONSE, build_handshake_response(False, "contract hash mismatch"))
            return
        send_frame(conn, MSG_HANDSHAKE_RESPONSE, build_handshake_response(True))
        while True:
            msg_type, payload = recv_frame(conn)
            if msg_type == MSG_CALL_REQUEST:
                try:
                    send_frame(conn, MSG_CALL_RESPONSE, handler(payload))
                except Exception as exc:
                    send_frame(conn, MSG_PLUGIN_ERROR, build_plugin_error(1, str(exc), False))
            elif msg_type == MSG_PING:
                send_frame(conn, MSG_PONG, build_pong(parse_ping_seq(payload)))
            elif msg_type == MSG_CANCEL:
                continue
            else:
                return
    except OSError:
        return
    finally:
        conn.close()


def serve(handler: Callable[[bytes], bytes], contract_hash: str) -> None:
    """Serve a plugin handler using PLUGIN_SOCKET or PLUGIN_ADDR from the environment."""
    sock_path = os.environ.get("PLUGIN_SOCKET", "")
    addr = os.environ.get("PLUGIN_ADDR", "")

    if sock_path:
        try:
            os.unlink(sock_path)
        except FileNotFoundError:
            pass
        server = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        server.bind(sock_path)
    elif addr:
        host, port = addr.rsplit(":", 1)
        server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        server.bind((host, int(port)))
    else:
        print("PLUGIN_SOCKET or PLUGIN_ADDR must be set", file=sys.stderr)
        raise SystemExit(1)

    server.listen(128)
    print("READY", flush=True)
    while True:
        conn, _ = server.accept()
        threading.Thread(target=_handle_conn, args=(conn, handler, contract_hash), daemon=True).start()
