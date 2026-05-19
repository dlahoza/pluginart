import * as net from 'node:net';
import {
  FrameReader,
  MSG_CALL_REQUEST,
  MSG_CALL_RESPONSE,
  MSG_CANCEL,
  MSG_HANDSHAKE_REQUEST,
  MSG_HANDSHAKE_RESPONSE,
  MSG_PING,
  MSG_PLUGIN_ERROR,
  MSG_PONG,
  buildHandshakeResponse,
  buildPluginError,
  buildPong,
  parseContractHash,
  parsePingSeq,
  sendFrame,
} from './protocol';

export type Handler = (payload: Buffer) => Buffer | Promise<Buffer>;

async function handleConn(sock: net.Socket, handler: Handler, contractHash: string): Promise<void> {
  const reader = new FrameReader();
  sock.on('data', (chunk: Buffer) => reader.feed(chunk));
  sock.on('error', (err) => reader.fail(err));
  sock.on('close', () => reader.fail(new Error('connection closed')));

  try {
    const hs = await reader.next();
    if (hs.type !== MSG_HANDSHAKE_REQUEST) return;
    if (parseContractHash(hs.payload) !== contractHash) {
      sendFrame(sock, MSG_HANDSHAKE_RESPONSE, buildHandshakeResponse(false, 'contract hash mismatch'));
      return;
    }
    sendFrame(sock, MSG_HANDSHAKE_RESPONSE, buildHandshakeResponse(true));
    while (true) {
      const frame = await reader.next();
      if (frame.type === MSG_CALL_REQUEST) {
        try {
          sendFrame(sock, MSG_CALL_RESPONSE, await handler(frame.payload));
        } catch (err) {
          sendFrame(sock, MSG_PLUGIN_ERROR, buildPluginError(1, err instanceof Error ? err.message : String(err)));
        }
      } else if (frame.type === MSG_PING) {
        sendFrame(sock, MSG_PONG, buildPong(parsePingSeq(frame.payload)));
      } else if (frame.type === MSG_CANCEL) {
        continue;
      } else {
        return;
      }
    }
  } catch {
    return;
  } finally {
    sock.destroy();
  }
}

export function serve(handler: Handler, options: { contractHash: string }): void {
  const sockPath = process.env.PLUGIN_SOCKET ?? '';
  const addr = process.env.PLUGIN_ADDR ?? '';
  const server = net.createServer((sock) => {
    void handleConn(sock, handler, options.contractHash);
  });
  if (sockPath) {
    server.listen(sockPath, () => process.stdout.write('READY\n'));
    return;
  }
  if (addr) {
    const colon = addr.lastIndexOf(':');
    server.listen(Number(addr.slice(colon + 1)), addr.slice(0, colon), () => process.stdout.write('READY\n'));
    return;
  }
  process.stderr.write('PLUGIN_SOCKET or PLUGIN_ADDR must be set\n');
  process.exit(1);
}
