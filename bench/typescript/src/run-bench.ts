import * as childProcess from 'node:child_process';
import * as fs from 'node:fs/promises';
import * as net from 'node:net';
import * as os from 'node:os';
import * as path from 'node:path';
import { performance } from 'node:perf_hooks';
import { PluginManager, FrameReader, connect } from '../../../runtimes/typescript/dist';
import type { Client } from '../../../runtimes/typescript/dist/protocol';

const ROOT = path.resolve(__dirname, '../../..');
const CONTRACT_HASH = 'sha256:pluginart-bench';
const PLUGIN_NAME = 'bench';
const SIZES = [10, 1000, 10000] as const;
const HOST_MEMORY_LIMIT = 16 * 1024 * 1024;
const PLUGIN_MEMORY_LIMIT = 32 * 1024 * 1024;

interface Result {
  runtime: string;
  benchmark: string;
  payload_bytes: number;
  iterations: number;
  ns_per_op: number;
  bytes_per_sec: number;
  heap_peak_bytes: number;
  heap_retained_bytes: number;
  heap_peak_bytes_per_op: number;
  heap_retained_bytes_per_op: number;
}

function freeTcpAddr(): Promise<string> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.listen(0, '127.0.0.1', () => {
      const addr = server.address();
      if (!addr || typeof addr === 'string') {
        reject(new Error('failed to allocate tcp port'));
        return;
      }
      server.close(() => resolve(`127.0.0.1:${addr.port}`));
    });
    server.on('error', reject);
  });
}

async function startTypeScriptPlugin(): Promise<{ addr: string; proc: childProcess.ChildProcess }> {
  const addr = await freeTcpAddr();
  const proc = childProcess.spawn('node', ['bench/typescript/dist/plugin-server.js'], {
    cwd: ROOT,
    env: { ...process.env, PLUGIN_ADDR: addr },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  await new Promise<void>((resolve, reject) => {
    const timer = setTimeout(() => {
      proc.kill();
      reject(new Error('typescript benchmark plugin startup timeout'));
    }, 15000);
    proc.stdout.on('data', (chunk: Buffer) => {
      if (chunk.toString().split(/\r?\n/).some((line) => line.trim() === 'READY')) {
        clearTimeout(timer);
        resolve();
      }
    });
    proc.once('exit', () => {
      clearTimeout(timer);
      reject(new Error('typescript benchmark plugin exited before READY'));
    });
  });
  return { addr, proc };
}

function stopProcess(proc: childProcess.ChildProcess): void {
  if (!proc.killed) proc.kill();
}

async function protocolClient(addr: string): Promise<Client> {
  const colon = addr.lastIndexOf(':');
  const sock = await new Promise<net.Socket>((resolve, reject) => {
    const s = net.createConnection({ host: addr.slice(0, colon), port: Number(addr.slice(colon + 1)) }, () => resolve(s));
    s.once('error', reject);
  });
  const reader = new FrameReader();
  sock.on('data', (chunk: Buffer) => reader.feed(chunk));
  sock.on('error', (err) => reader.fail(err));
  sock.on('close', () => reader.fail(new Error('connection closed')));
  return connect(sock, reader, PLUGIN_NAME, CONTRACT_HASH);
}

async function managerClient(addr: string): Promise<PluginManager> {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'pluginart-bench-'));
  const configPath = path.join(dir, 'pluginart.toml');
  await fs.writeFile(configPath, `version = 1

[defaults]
startup_timeout = "5s"
shutdown_timeout = "2s"
health_interval = "30s"
max_restarts = 0

[[plugins]]
name = "${PLUGIN_NAME}"
type = "remote"
address = "${addr}"
contract_hash = "${CONTRACT_HASH}"
`);
  const manager = await PluginManager.fromConfig(configPath);
  await manager.start();
  return manager;
}

async function timeCalls(
  benchmark: string,
  payloadSize: number,
  durationMs: number,
  call: (payload: Buffer) => Promise<Buffer>,
  iterations?: number,
): Promise<Result> {
  const payload = Buffer.alloc(payloadSize, 'x');
  if (global.gc) global.gc();
  const before = process.memoryUsage().heapUsed;
  let peak = before;
  const start = performance.now();
  let calls = 0;
  const deadline = start + durationMs;
  while (iterations === undefined || calls < iterations) {
    const resp = await call(payload);
    if (resp.length !== payload.length) {
      throw new Error(`response size ${resp.length} != ${payload.length}`);
    }
    calls += 1;
    peak = Math.max(peak, process.memoryUsage().heapUsed);
    if (iterations === undefined && calls >= 1 && performance.now() >= deadline) break;
  }
  const elapsedNs = Math.round((performance.now() - start) * 1_000_000);
  if (global.gc) global.gc();
  const after = process.memoryUsage().heapUsed;
  const retained = Math.max(0, after - before);
  const peakDelta = Math.max(0, peak - before);
  return {
    runtime: 'typescript',
    benchmark,
    payload_bytes: payloadSize,
    iterations: calls,
    ns_per_op: Math.round(elapsedNs / calls),
    bytes_per_sec: Math.round((payloadSize * calls * 1_000_000_000) / elapsedNs),
    heap_peak_bytes: peakDelta,
    heap_retained_bytes: retained,
    heap_peak_bytes_per_op: peakDelta / calls,
    heap_retained_bytes_per_op: retained / calls,
  };
}

async function assertMemoryGrowth(call: (payload: Buffer) => Promise<Buffer>): Promise<void> {
  if (!global.gc) {
    throw new Error('run TypeScript benchmarks with node --expose-gc');
  }
  const payload = Buffer.alloc(10000, 'x');
  for (let i = 0; i < 10; i += 1) await call(payload);
  global.gc();
  const before = process.memoryUsage().heapUsed;
  for (let i = 0; i < 500; i += 1) {
    const resp = await call(payload);
    if (resp.length !== payload.length) {
      throw new Error(`response size ${resp.length} != ${payload.length}`);
    }
  }
  global.gc();
  const after = process.memoryUsage().heapUsed;
  const growth = after - before;
  if (growth > HOST_MEMORY_LIMIT) {
    throw new Error(`typescript heap grew by ${growth} bytes, want <= ${HOST_MEMORY_LIMIT}`);
  }
}

async function processRss(pid: number): Promise<number | null> {
  try {
    const status = await fs.readFile(`/proc/${pid}/status`, 'utf8');
    const line = status.split(/\r?\n/).find((candidate) => candidate.startsWith('VmRSS:'));
    if (line) return Number(line.trim().split(/\s+/)[1]) * 1024;
  } catch {
    // Fall through to ps for non-Linux systems.
  }
  return new Promise((resolve) => {
    childProcess.execFile('ps', ['-o', 'rss=', '-p', String(pid)], (err, stdout) => {
      if (err) {
        resolve(null);
        return;
      }
      const kb = Number(stdout.trim());
      resolve(Number.isFinite(kb) ? kb * 1024 : null);
    });
  });
}

async function assertPluginMemoryGrowth(pid: number, call: (payload: Buffer) => Promise<Buffer>): Promise<void> {
  const payload = Buffer.alloc(10000, 'x');
  for (let i = 0; i < 10; i += 1) await call(payload);
  const before = await processRss(pid);
  if (before === null) return;
  for (let i = 0; i < 500; i += 1) {
    const resp = await call(payload);
    if (resp.length !== payload.length) {
      throw new Error(`response size ${resp.length} != ${payload.length}`);
    }
  }
  const after = await processRss(pid);
  if (after === null) return;
  const growth = after - before;
  if (growth > PLUGIN_MEMORY_LIMIT) {
    throw new Error(`typescript plugin RSS grew by ${growth} bytes, want <= ${PLUGIN_MEMORY_LIMIT}`);
  }
}

function printTable(results: Result[]): void {
  console.log('| Benchmark | Payload | Calls | ns/op | MB/s | Peak heap | Retained heap | Peak heap/op | Retained heap/op |');
  console.log('| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |');
  for (const result of results) {
    const mbps = result.bytes_per_sec / (1024 * 1024);
    console.log(
      `| ${result.benchmark} | ${result.payload_bytes} | ${result.iterations} | ${result.ns_per_op} | ${mbps.toFixed(2)} | ` +
      `${result.heap_peak_bytes} B | ${result.heap_retained_bytes} B | ` +
      `${result.heap_peak_bytes_per_op.toFixed(2)} B/op | ${result.heap_retained_bytes_per_op.toFixed(2)} B/op |`,
    );
  }
}

function argValue(name: string): string | undefined {
  const index = process.argv.indexOf(name);
  return index >= 0 ? process.argv[index + 1] : undefined;
}

function parseDuration(value: string): number {
  if (value.endsWith('ms')) return Number(value.slice(0, -2));
  if (value.endsWith('s')) return Number(value.slice(0, -1)) * 1000;
  if (value.endsWith('m')) return Number(value.slice(0, -1)) * 60_000;
  return Number(value);
}

async function main(): Promise<void> {
  const iterationsArg = argValue('--iterations');
  const iterations = iterationsArg === undefined ? undefined : Number(iterationsArg);
  const durationMs = parseDuration(argValue('--duration') ?? '10s');
  const jsonPath = argValue('--json');
  const { addr, proc } = await startTypeScriptPlugin();
  const results: Result[] = [];
  try {
    const client = await protocolClient(addr);
    try {
      for (const size of SIZES) {
        results.push(await timeCalls('protocol_client', size, durationMs, (payload) => client.call(payload), iterations));
      }
      await assertMemoryGrowth((payload) => client.call(payload));
    } finally {
      client.close();
    }

    const manager = await managerClient(addr);
    try {
      for (const size of SIZES) {
        results.push(await timeCalls('plugin_manager', size, durationMs, (payload) => manager.call(PLUGIN_NAME, payload), iterations));
      }
      await assertMemoryGrowth((payload) => manager.call(PLUGIN_NAME, payload));
    } finally {
      await manager.shutdown();
    }

    const serverClient = await protocolClient(addr);
    try {
      for (const size of SIZES) {
        results.push(await timeCalls('plugin_server', size, durationMs, (payload) => serverClient.call(payload), iterations));
      }
      if (proc.pid !== undefined) {
        await assertPluginMemoryGrowth(proc.pid, (payload) => serverClient.call(payload));
      }
    } finally {
      serverClient.close();
    }
  } finally {
    stopProcess(proc);
  }

  printTable(results);
  if (jsonPath) {
    await fs.mkdir(path.dirname(jsonPath), { recursive: true });
    await fs.writeFile(jsonPath, `${JSON.stringify(results, null, 2)}\n`);
  }
}

main().catch((err: unknown) => {
  console.error(err);
  process.exit(1);
});
