import * as child_process from 'child_process';
import * as net from 'net';
import * as os from 'os';
import * as path from 'path';
import { connectTo, callPlugin, FrameReader } from './pluginart_wire';

export interface PluginHandle {
  sock: net.Socket;
  reader: FrameReader;
  proc: child_process.ChildProcess;
}

export async function spawnPlugin(
  cmd: string,
  args: string[],
  label: string,
): Promise<PluginHandle> {
  const sockPath = path.join(os.tmpdir(), `pluginart-${label}-${process.pid}.sock`);

  const proc = child_process.spawn(cmd, args, {
    env: { ...process.env, PLUGIN_SOCKET: sockPath },
    stdio: ['ignore', 'pipe', 'inherit'],
  });

  await waitReady(proc, label);

  const { sock, reader } = await connectTo({ sockPath }, label);
  return { sock, reader, proc };
}

function waitReady(proc: child_process.ChildProcess, label: string): Promise<void> {
  return new Promise((resolve, reject) => {
    proc.once('error', reject);
    proc.once('exit', (code) => reject(new Error(`${label} exited early with code ${code}`)));

    let buf = '';
    proc.stdout!.on('data', (chunk: Buffer) => {
      buf += chunk.toString();
      if (buf.includes('READY')) resolve();
    });
  });
}

export async function echo(handle: PluginHandle, requestBytes: Buffer): Promise<Buffer> {
  return callPlugin(handle.sock, handle.reader, requestBytes);
}

export function kill(handle: PluginHandle): void {
  handle.sock.destroy();
  handle.proc.kill();
}
