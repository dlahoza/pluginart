import * as net from 'node:net';
import * as flatbuffers from 'flatbuffers';
import { HandshakeError, PluginError, TransportError } from './errors';

export const MAGIC = Buffer.from([0x50, 0x4c, 0x47, 0x4e]);
export const HEADER_SIZE = 9;
export const MAX_FRAME_SIZE = 4 * 1024 * 1024;

export const MSG_HANDSHAKE_REQUEST = 0x01;
export const MSG_HANDSHAKE_RESPONSE = 0x02;
export const MSG_CALL_REQUEST = 0x03;
export const MSG_CALL_RESPONSE = 0x04;
export const MSG_PLUGIN_ERROR = 0x05;
export const MSG_CANCEL = 0x06;
export const MSG_PING = 0x07;
export const MSG_PONG = 0x08;

export interface Frame {
  type: number;
  payload: Buffer;
}

export class FrameReader {
  private buffer = Buffer.alloc(0);
  private pending: Array<{ resolve: (frame: Frame) => void; reject: (err: Error) => void }> = [];
  private error: Error | null = null;

  feed(chunk: Buffer): void {
    if (this.error) return;
    this.buffer = Buffer.concat([this.buffer, chunk]);
    this.flush();
  }

  fail(err: Error): void {
    this.error = err;
    for (const waiter of this.pending.splice(0)) waiter.reject(err);
  }

  next(): Promise<Frame> {
    return new Promise((resolve, reject) => {
      this.pending.push({ resolve, reject });
      this.flush();
    });
  }

  private flush(): void {
    while (this.pending.length > 0 && this.buffer.length >= HEADER_SIZE) {
      if (!this.buffer.subarray(0, 4).equals(MAGIC)) {
        this.fail(new TransportError('invalid magic bytes'));
        return;
      }
      const length = this.buffer.readUInt32LE(4);
      if (length > MAX_FRAME_SIZE) {
        this.fail(new TransportError(`frame length ${length} exceeds max frame size`));
        return;
      }
      if (this.buffer.length < HEADER_SIZE + length) return;
      const type = this.buffer.readUInt8(8);
      const payload = Buffer.from(this.buffer.subarray(HEADER_SIZE, HEADER_SIZE + length));
      this.buffer = this.buffer.subarray(HEADER_SIZE + length);
      this.pending.shift()!.resolve({ type, payload });
    }
  }
}

export function sendFrame(sock: net.Socket, type: number, payload: Buffer): void {
  if (payload.length > MAX_FRAME_SIZE) {
    throw new TransportError(`payload ${payload.length} bytes exceeds max frame size`);
  }
  const hdr = Buffer.allocUnsafe(HEADER_SIZE);
  MAGIC.copy(hdr, 0);
  hdr.writeUInt32LE(payload.length, 4);
  hdr.writeUInt8(type, 8);
  sock.write(Buffer.concat([hdr, payload]));
}

function table(payload: Buffer): { bb: flatbuffers.ByteBuffer; pos: number } {
  const bb = new flatbuffers.ByteBuffer(new Uint8Array(payload));
  return { bb, pos: bb.readInt32(bb.position()) + bb.position() };
}

export function buildHandshakeRequest(contractHash: string, pluginName: string): Buffer {
  const builder = new flatbuffers.Builder(128);
  const ch = builder.createString(contractHash);
  const pn = builder.createString(pluginName);
  builder.startObject(3);
  builder.addFieldOffset(0, ch, 0);
  builder.addFieldOffset(1, pn, 0);
  builder.addFieldInt16(2, 1, 0);
  const root = builder.endObject();
  builder.finish(root);
  return Buffer.from(builder.asUint8Array());
}

export function buildHandshakeResponse(ok: boolean, error = ''): Buffer {
  const builder = new flatbuffers.Builder(128);
  const err = error ? builder.createString(error) : 0;
  builder.startObject(2);
  builder.addFieldInt8(0, ok ? 1 : 0, 0);
  if (err) builder.addFieldOffset(1, err, 0);
  const root = builder.endObject();
  builder.finish(root);
  return Buffer.from(builder.asUint8Array());
}

export function buildPing(seq: bigint): Buffer {
  const builder = new flatbuffers.Builder(32);
  builder.startObject(1);
  builder.addFieldInt64(0, seq, BigInt(0));
  const root = builder.endObject();
  builder.finish(root);
  return Buffer.from(builder.asUint8Array());
}

export const buildPong = buildPing;

export function buildPluginError(code: number, message: string, retry = false): Buffer {
  const builder = new flatbuffers.Builder(128);
  const msg = builder.createString(message);
  builder.startObject(3);
  builder.addFieldInt16(0, code, 0);
  builder.addFieldOffset(1, msg, 0);
  builder.addFieldInt8(2, retry ? 1 : 0, 0);
  const root = builder.endObject();
  builder.finish(root);
  return Buffer.from(builder.asUint8Array());
}

export function parseContractHash(payload: Buffer): string {
  const { bb, pos } = table(payload);
  const off = bb.__offset(pos, 4);
  return off ? ((bb.__string(pos + off) as string) ?? '') : '';
}

export function parseHandshakeResponse(payload: Buffer): { ok: boolean; error: string } {
  const { bb, pos } = table(payload);
  const okOff = bb.__offset(pos, 4);
  const errOff = bb.__offset(pos, 6);
  return {
    ok: okOff ? bb.readInt8(pos + okOff) !== 0 : false,
    error: errOff ? ((bb.__string(pos + errOff) as string) ?? '') : '',
  };
}

export function parsePingSeq(payload: Buffer): bigint {
  const { bb, pos } = table(payload);
  const off = bb.__offset(pos, 4);
  return off ? bb.readInt64(pos + off) : BigInt(0);
}

export function parsePluginError(payload: Buffer): { code: number; message: string; retry: boolean } {
  const { bb, pos } = table(payload);
  const codeOff = bb.__offset(pos, 4);
  const msgOff = bb.__offset(pos, 6);
  const retryOff = bb.__offset(pos, 8);
  return {
    code: codeOff ? bb.readInt16(pos + codeOff) : 0,
    message: msgOff ? ((bb.__string(pos + msgOff) as string) ?? '') : '',
    retry: retryOff ? bb.readInt8(pos + retryOff) !== 0 : false,
  };
}

export class Client {
  private seq = BigInt(0);
  private queue: Promise<unknown> = Promise.resolve();

  constructor(
    private sock: net.Socket,
    private reader: FrameReader,
    readonly pluginName: string,
    readonly contractHash: string,
  ) {}

  async call(payload: Buffer): Promise<Buffer> {
    return this.locked(() => call(this.sock, this.reader, payload));
  }

  async ping(): Promise<void> {
    return this.locked(async () => {
      this.seq += BigInt(1);
      sendFrame(this.sock, MSG_PING, buildPing(this.seq));
      while (true) {
        const frame = await this.reader.next();
        if (frame.type === MSG_PONG && parsePingSeq(frame.payload) === this.seq) return;
      }
    });
  }

  close(): void {
    this.sock.destroy();
  }

  private locked<T>(fn: () => Promise<T>): Promise<T> {
    const run = this.queue.then(fn, fn);
    this.queue = run.catch(() => undefined);
    return run;
  }
}

export async function connect(
  sock: net.Socket,
  reader: FrameReader,
  pluginName: string,
  contractHash: string,
): Promise<Client> {
  sendFrame(sock, MSG_HANDSHAKE_REQUEST, buildHandshakeRequest(contractHash, pluginName));
  const frame = await reader.next();
  if (frame.type !== MSG_HANDSHAKE_RESPONSE) {
    throw new HandshakeError(`expected handshake response, got ${frame.type}`);
  }
  const resp = parseHandshakeResponse(frame.payload);
  if (!resp.ok) throw new HandshakeError(`handshake rejected: ${resp.error}`);
  return new Client(sock, reader, pluginName, contractHash);
}

export async function call(sock: net.Socket, reader: FrameReader, payload: Buffer): Promise<Buffer> {
  sendFrame(sock, MSG_CALL_REQUEST, payload);
  while (true) {
    const frame = await reader.next();
    if (frame.type === MSG_CALL_RESPONSE) return frame.payload;
    if (frame.type === MSG_PLUGIN_ERROR) {
      const err = parsePluginError(frame.payload);
      throw new PluginError(`plugin error ${err.code}: ${err.message}`);
    }
    if (frame.type !== MSG_PING && frame.type !== MSG_PONG) {
      throw new TransportError(`unexpected message type ${frame.type}`);
    }
  }
}
