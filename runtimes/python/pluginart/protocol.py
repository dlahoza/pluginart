from __future__ import annotations

import socket
import struct
import threading

import flatbuffers
import flatbuffers.encode
import flatbuffers.packer
import flatbuffers.table

from .errors import HandshakeError, PluginError, TransportError

MAGIC = b"PLGN"
HEADER_SIZE = 9
MAX_FRAME_SIZE = 4 * 1024 * 1024

MSG_HANDSHAKE_REQUEST = 0x01
MSG_HANDSHAKE_RESPONSE = 0x02
MSG_CALL_REQUEST = 0x03
MSG_CALL_RESPONSE = 0x04
MSG_PLUGIN_ERROR = 0x05
MSG_CANCEL = 0x06
MSG_PING = 0x07
MSG_PONG = 0x08


def _read_exact(sock: socket.socket, size: int) -> bytes:
    buf = bytearray()
    while len(buf) < size:
        chunk = sock.recv(size - len(buf))
        if not chunk:
            raise TransportError("connection closed")
        buf.extend(chunk)
    return bytes(buf)


def recv_frame(sock: socket.socket) -> tuple[int, bytes]:
    hdr = _read_exact(sock, HEADER_SIZE)
    if hdr[:4] != MAGIC:
        raise TransportError("invalid magic bytes")
    length, msg_type = struct.unpack("<IB", hdr[4:])
    if length > MAX_FRAME_SIZE:
        raise TransportError(f"frame length {length} exceeds max frame size")
    return msg_type, _read_exact(sock, length)


def send_frame(sock: socket.socket, msg_type: int, payload: bytes) -> None:
    if len(payload) > MAX_FRAME_SIZE:
        raise TransportError(f"payload {len(payload)} bytes exceeds max frame size")
    sock.sendall(MAGIC + struct.pack("<IB", len(payload), msg_type) + payload)


def build_handshake_request(contract_hash: str, plugin_name: str) -> bytes:
    b = flatbuffers.Builder(128)
    ch = b.CreateString(contract_hash)
    pn = b.CreateString(plugin_name)
    b.StartObject(3)
    b.PrependUOffsetTRelativeSlot(0, ch, 0)
    b.PrependUOffsetTRelativeSlot(1, pn, 0)
    b.PrependUint16Slot(2, 1, 0)
    root = b.EndObject()
    b.Finish(root)
    return bytes(b.Output())


def build_handshake_response(ok: bool, error: str = "") -> bytes:
    b = flatbuffers.Builder(128)
    err = b.CreateString(error) if error else 0
    b.StartObject(2)
    b.PrependBoolSlot(0, ok, 0)
    if err:
        b.PrependUOffsetTRelativeSlot(1, err, 0)
    root = b.EndObject()
    b.Finish(root)
    return bytes(b.Output())


def build_ping(seq: int) -> bytes:
    b = flatbuffers.Builder(32)
    b.StartObject(1)
    b.PrependUint64Slot(0, seq, 0)
    root = b.EndObject()
    b.Finish(root)
    return bytes(b.Output())


def build_pong(seq: int) -> bytes:
    return build_ping(seq)


def build_plugin_error(code: int, message: str, retry: bool = False) -> bytes:
    b = flatbuffers.Builder(128)
    msg = b.CreateString(message)
    b.StartObject(3)
    b.PrependUint16Slot(0, code, 0)
    b.PrependUOffsetTRelativeSlot(1, msg, 0)
    b.PrependBoolSlot(2, retry, 0)
    root = b.EndObject()
    b.Finish(root)
    return bytes(b.Output())


def _table(buf: bytes) -> flatbuffers.table.Table:
    ba = bytearray(buf)
    root_pos = flatbuffers.encode.Get(flatbuffers.packer.uoffset, ba, 0)
    return flatbuffers.table.Table(ba, root_pos)


def parse_contract_hash(buf: bytes) -> str:
    tab = _table(buf)
    off = tab.Offset(4)
    return tab.String(off + tab.Pos).decode("utf-8") if off else ""


def parse_handshake_response(buf: bytes) -> tuple[bool, str]:
    tab = _table(buf)
    ok_off = tab.Offset(4)
    ok = bool(tab.Bytes[tab.Pos + ok_off]) if ok_off else False
    err_off = tab.Offset(6)
    err = tab.String(err_off + tab.Pos).decode("utf-8") if err_off else ""
    return ok, err


def parse_ping_seq(buf: bytes) -> int:
    tab = _table(buf)
    off = tab.Offset(4)
    return flatbuffers.encode.Get(flatbuffers.packer.uint64, tab.Bytes, tab.Pos + off) if off else 0


def parse_plugin_error(buf: bytes) -> tuple[int, str, bool]:
    tab = _table(buf)
    code_off = tab.Offset(4)
    code = flatbuffers.encode.Get(flatbuffers.packer.uint16, tab.Bytes, tab.Pos + code_off) if code_off else 0
    msg_off = tab.Offset(6)
    msg = tab.String(msg_off + tab.Pos).decode("utf-8") if msg_off else ""
    retry_off = tab.Offset(8)
    retry = bool(tab.Bytes[tab.Pos + retry_off]) if retry_off else False
    return code, msg, retry


class Client:
    def __init__(self, sock: socket.socket, plugin_name: str, contract_hash: str) -> None:
        self._sock = sock
        self._lock = threading.Lock()
        self.plugin_name = plugin_name
        self.contract_hash = contract_hash

    def call(self, payload: bytes) -> bytes:
        return call(self._sock, payload, self._lock)

    def ping(self, seq: int) -> None:
        with self._lock:
            send_frame(self._sock, MSG_PING, build_ping(seq))
            while True:
                msg_type, data = recv_frame(self._sock)
                if msg_type == MSG_PONG and parse_ping_seq(data) == seq:
                    return

    def close(self) -> None:
        self._sock.close()


def connect(sock: socket.socket, plugin_name: str, contract_hash: str) -> Client:
    send_frame(sock, MSG_HANDSHAKE_REQUEST, build_handshake_request(contract_hash, plugin_name))
    msg_type, payload = recv_frame(sock)
    if msg_type != MSG_HANDSHAKE_RESPONSE:
        raise HandshakeError(f"expected handshake response, got {msg_type}")
    ok, err = parse_handshake_response(payload)
    if not ok:
        raise HandshakeError(f"handshake rejected: {err}")
    return Client(sock, plugin_name, contract_hash)


def call(sock: socket.socket, payload: bytes, lock: threading.Lock | None = None) -> bytes:
    def round_trip() -> bytes:
        send_frame(sock, MSG_CALL_REQUEST, payload)
        while True:
            msg_type, resp = recv_frame(sock)
            if msg_type == MSG_CALL_RESPONSE:
                return resp
            if msg_type == MSG_PLUGIN_ERROR:
                code, message, retry = parse_plugin_error(resp)
                raise PluginError(f"plugin error {code}: {message}", retry)
            if msg_type not in (MSG_PING, MSG_PONG):
                raise TransportError(f"unexpected message type {msg_type}")

    if lock is None:
        return round_trip()
    with lock:
        return round_trip()
