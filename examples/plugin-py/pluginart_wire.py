"""pluginart wire protocol: framing, FlatBuffers handshake, serve() and connect helpers."""
import os
import socket
import struct
import sys
import threading

import flatbuffers
import flatbuffers.encode
import flatbuffers.packer
import flatbuffers.table

_MAGIC    = b'PLGN'
_HDR_SIZE = 9  # 4 magic + 4 length + 1 flags

_MSG_HANDSHAKE_REQUEST  = 0x01
_MSG_HANDSHAKE_RESPONSE = 0x02
_MSG_CALL_REQUEST       = 0x03
_MSG_CALL_RESPONSE      = 0x04
_MSG_PLUGIN_ERROR       = 0x05
_MSG_CANCEL             = 0x06
_MSG_PING               = 0x07
_MSG_PONG               = 0x08

CONTRACT_HASH = "sha256:094f99745014e0e307ad2b73394a45887059d3ce7fcda59fbd741f77e7904a14"


def _recv_frame(sock: socket.socket) -> tuple[int, bytes]:
    hdr = b''
    while len(hdr) < _HDR_SIZE:
        chunk = sock.recv(_HDR_SIZE - len(hdr))
        if not chunk:
            raise ConnectionError('connection closed')
        hdr += chunk
    if hdr[:4] != _MAGIC:
        raise ValueError(f'bad magic: {hdr[:4]!r}')
    length, flags = struct.unpack('<IB', hdr[4:])
    payload = b''
    while len(payload) < length:
        chunk = sock.recv(length - len(payload))
        if not chunk:
            raise ConnectionError('connection closed mid-payload')
        payload += chunk
    return flags, payload


def _send_frame(sock: socket.socket, flags: int, payload: bytes) -> None:
    hdr = _MAGIC + struct.pack('<IB', len(payload), flags)
    sock.sendall(hdr + payload)


def _parse_contract_hash(buf: bytes) -> str:
    ba = bytearray(buf)
    root_pos = flatbuffers.encode.Get(flatbuffers.packer.uoffset, ba, 0)
    tab = flatbuffers.table.Table(ba, root_pos)
    o = tab.Offset(4)
    return tab.String(o + tab.Pos).decode('utf-8') if o else ''


def _build_handshake_response(ok: bool, error: str = '') -> bytes:
    b = flatbuffers.Builder(64)
    err_off = b.CreateString(error) if error else 0
    b.StartObject(2)
    b.PrependBoolSlot(0, ok, 0)
    if err_off:
        b.PrependUOffsetTRelativeSlot(1, err_off, 0)
    root = b.EndObject()
    b.Finish(root)
    return bytes(b.Output())


def _handle_conn(conn: socket.socket, handler) -> None:
    try:
        flags, payload = _recv_frame(conn)
        if flags != _MSG_HANDSHAKE_REQUEST:
            return
        if _parse_contract_hash(payload) != CONTRACT_HASH:
            got = _parse_contract_hash(payload)
            _send_frame(conn, _MSG_HANDSHAKE_RESPONSE,
                        _build_handshake_response(False, f'contract mismatch: got {got!r}'))
            return
        _send_frame(conn, _MSG_HANDSHAKE_RESPONSE, _build_handshake_response(True))
        while True:
            flags, payload = _recv_frame(conn)
            if flags == _MSG_CALL_REQUEST:
                try:
                    _send_frame(conn, _MSG_CALL_RESPONSE, handler(payload))
                except Exception as exc:
                    _send_frame(conn, _MSG_PLUGIN_ERROR, str(exc).encode())
            elif flags == _MSG_PING:
                _send_frame(conn, _MSG_PONG, payload)
            elif flags == _MSG_CANCEL:
                pass
            else:
                break
    except (ConnectionError, OSError):
        pass
    finally:
        conn.close()


def serve(handler) -> None:
    """Start the plugin server. handler(payload: bytes) -> bytes."""
    sock_path = os.environ.get('PLUGIN_SOCKET', '')
    addr_str  = os.environ.get('PLUGIN_ADDR', '')

    if sock_path:
        srv = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        srv.bind(sock_path)
    elif addr_str:
        host, port = addr_str.rsplit(':', 1)
        srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        srv.bind((host, int(port)))
    else:
        print('PLUGIN_SOCKET or PLUGIN_ADDR must be set', file=sys.stderr)
        sys.exit(1)

    srv.listen(128)
    print('READY', flush=True)

    while True:
        conn, _ = srv.accept()
        threading.Thread(target=_handle_conn, args=(conn, handler), daemon=True).start()
