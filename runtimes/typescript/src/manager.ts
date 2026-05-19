import * as childProcess from 'node:child_process';
import * as fs from 'node:fs/promises';
import * as net from 'node:net';
import * as os from 'node:os';
import * as path from 'node:path';
import { parse } from 'smol-toml';
import { Client, FrameReader, connect } from './protocol';
import { ConfigError, StartupError, TransportError } from './errors';

type PluginConfig = Record<string, any>;

function duration(value: unknown, fallback: number): number {
  if (value === undefined || value === null || value === '') return fallback;
  if (typeof value === 'number') return value * 1000;
  if (typeof value !== 'string') throw new ConfigError(`invalid duration ${String(value)}`);
  const match = value.match(/^([0-9.]+)(ms|s|m|h)?$/);
  if (!match) throw new ConfigError(`invalid duration ${value}`);
  const n = Number(match[1]);
  const units: Record<string, number> = { ms: 1, s: 1000, m: 60000, h: 3600000 };
  return n * units[match[2] ?? 's'];
}

function freeTcpAddr(): Promise<string> {
  return new Promise((resolve, reject) => {
    const server = net.createServer();
    server.listen(0, '127.0.0.1', () => {
      const addr = server.address();
      if (!addr || typeof addr === 'string') {
        reject(new TransportError('failed to allocate tcp port'));
        return;
      }
      server.close(() => resolve(`127.0.0.1:${addr.port}`));
    });
    server.on('error', reject);
  });
}

function unixPath(): string {
  return path.join(os.tmpdir(), 'pluginart', `${process.pid}-${Date.now()}-${Math.random().toString(16).slice(2)}.sock`);
}

async function openSocket(kind: 'unix' | 'tcp', addr: string): Promise<{ sock: net.Socket; reader: FrameReader }> {
  const sock = await new Promise<net.Socket>((resolve, reject) => {
    const s = kind === 'unix'
      ? net.createConnection({ path: addr }, () => resolve(s))
      : net.createConnection({ host: addr.slice(0, addr.lastIndexOf(':')), port: Number(addr.slice(addr.lastIndexOf(':') + 1)) }, () => resolve(s));
    s.once('error', reject);
  });
  const reader = new FrameReader();
  sock.on('data', (chunk: Buffer) => reader.feed(chunk));
  sock.on('error', (err) => reader.fail(err));
  sock.on('close', () => reader.fail(new Error('connection closed')));
  return { sock, reader };
}

class ManagedPlugin {
  private client: Client | null = null;
  private proc: childProcess.ChildProcess | null = null;
  private containerId = '';
  private healthTimer: NodeJS.Timeout | null = null;
  private restartCount = 0;
  private restartBackoffMs = 1000;

  constructor(private cfg: PluginConfig, private defaults: PluginConfig) {}

  get name(): string {
    return String(this.cfg.name ?? '');
  }

  async start(): Promise<void> {
    const type = this.cfg.type ?? 'binary';
    if (type === 'binary') await this.startBinary();
    else if (type === 'remote') await this.startRemote();
    else if (type === 'docker') await this.startDocker();
    else throw new ConfigError(`unsupported plugin type ${type}`);
    this.startHealth();
  }

  private transport(): Promise<{ kind: 'unix' | 'tcp'; addr: string }> {
    const type = this.cfg.type ?? 'binary';
    if (this.cfg.transport === 'tcp' || type === 'remote' || type === 'docker') {
      if (type === 'remote') return Promise.resolve({ kind: 'tcp', addr: String(this.cfg.address) });
      return freeTcpAddr().then((addr) => ({ kind: 'tcp', addr }));
    }
    return fs.mkdir(path.dirname(unixPath()), { recursive: true }).then(() => ({ kind: 'unix', addr: unixPath() }));
  }

  private async connect(kind: 'unix' | 'tcp', addr: string): Promise<void> {
    const { sock, reader } = await openSocket(kind, addr);
    this.client = await connect(sock, reader, this.name, String(this.cfg.contract_hash ?? ''));
  }

  private async startBinary(): Promise<void> {
    const { kind, addr } = await this.transport();
    const env = { ...process.env, ...(this.cfg.env ?? {}) };
    env[kind === 'tcp' ? 'PLUGIN_ADDR' : 'PLUGIN_SOCKET'] = addr;
    this.proc = childProcess.spawn(String(this.cfg.path), (this.cfg.args ?? []).map(String), {
      env,
      stdio: ['ignore', 'pipe', 'inherit'],
    });
    await this.waitReady(this.proc, duration(this.cfg.startup_timeout, duration(this.defaults.startup_timeout, 5000)));
    await this.connect(kind, addr);
  }

  private async startRemote(): Promise<void> {
    if (!this.cfg.address) throw new ConfigError(`remote plugin ${this.name} requires address`);
    await this.connect('tcp', String(this.cfg.address));
  }

  private async startDocker(): Promise<void> {
    const addr = await freeTcpAddr();
    const colon = addr.lastIndexOf(':');
    const host = addr.slice(0, colon);
    const port = addr.slice(colon + 1);
    const containerAddr = `0.0.0.0:${port}`;
    const args = ['run', '-d', '-p', `${host}:${port}:${port}`, '-e', `PLUGIN_ADDR=${containerAddr}`];
    for (const [k, v] of Object.entries(this.cfg.env ?? {})) args.push('-e', `${k}=${v}`);
    if (this.cfg.resources?.memory) args.push('--memory', String(this.cfg.resources.memory));
    if (this.cfg.resources?.cpus) args.push('--cpus', String(this.cfg.resources.cpus));
    args.push(String(this.cfg.image), ...(this.cfg.args ?? []).map(String));
    this.containerId = await new Promise((resolve, reject) => {
      childProcess.execFile('docker', args, (err, stdout) => err ? reject(err) : resolve(stdout.trim()));
    });
    const logs = childProcess.spawn('docker', ['logs', '-f', this.containerId], { stdio: ['ignore', 'pipe', 'ignore'] });
    await this.waitReady(logs, duration(this.cfg.startup_timeout, duration(this.defaults.startup_timeout, 5000)));
    await this.connect('tcp', addr);
  }

  private waitReady(proc: childProcess.ChildProcess, timeoutMs: number): Promise<void> {
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        proc.kill();
        reject(new StartupError(`plugin ${this.name} startup timeout`));
      }, timeoutMs);
      proc.once('exit', () => {
        clearTimeout(timer);
        reject(new StartupError(`plugin ${this.name} exited before READY`));
      });
      proc.stdout?.on('data', (chunk: Buffer) => {
        if (chunk.toString().split(/\r?\n/).some((line) => line.trim() === 'READY')) {
          clearTimeout(timer);
          resolve();
        }
      });
    });
  }

  private startHealth(): void {
    const interval = duration(this.cfg.health_interval, duration(this.defaults.health_interval, 2000));
    this.healthTimer = setInterval(() => {
      void this.client?.ping()
        .then(() => {
          this.restartBackoffMs = 1000;
        })
        .catch(() => this.restartAfterHealthFailure());
    }, interval);
  }

  private async restartAfterHealthFailure(): Promise<void> {
    if ((this.cfg.type ?? 'binary') !== 'binary') return;
    const maxRestarts = Number(this.cfg.max_restarts ?? this.defaults.max_restarts ?? 5);
    if (this.restartCount >= maxRestarts) return;
    this.restartCount += 1;
    const wait = this.restartBackoffMs;
    this.restartBackoffMs = Math.min(
      this.restartBackoffMs * 2,
      duration(this.cfg.restart_backoff_max, duration(this.defaults.restart_backoff_max, 30000)),
    );
    await new Promise((resolve) => setTimeout(resolve, wait));
    this.client?.close();
    this.client = null;
    if (this.proc && !this.proc.killed) this.proc.kill('SIGTERM');
    await this.startBinary();
  }

  async call(payload: Buffer): Promise<Buffer> {
    if (!this.client) throw new TransportError(`plugin ${this.name} not connected`);
    return this.client.call(payload);
  }

  async shutdown(): Promise<void> {
    if (this.healthTimer) clearInterval(this.healthTimer);
    this.client?.close();
    this.client = null;
    if (this.containerId) {
      childProcess.spawnSync('docker', ['stop', '-t', '1', this.containerId]);
      childProcess.spawnSync('docker', ['rm', this.containerId]);
      this.containerId = '';
    }
    if (this.proc && !this.proc.killed) {
      this.proc.kill('SIGTERM');
    }
  }
}

export class PluginManager {
  private constructor(private plugins: Map<string, ManagedPlugin>) {}

  static async fromConfig(configPath: string): Promise<PluginManager> {
    const cfg = parse(await fs.readFile(configPath, 'utf8')) as any;
    if (cfg.version !== 1) throw new ConfigError(`unsupported config version ${cfg.version}`);
    const plugins = new Map<string, ManagedPlugin>();
    for (const pcfg of cfg.plugins ?? []) {
      if (!pcfg.name) throw new ConfigError('plugin name is required');
      if (plugins.has(pcfg.name)) throw new ConfigError(`duplicate plugin name ${pcfg.name}`);
      plugins.set(pcfg.name, new ManagedPlugin(pcfg, cfg.defaults ?? {}));
    }
    return new PluginManager(plugins);
  }

  async start(): Promise<void> {
    for (const plugin of this.plugins.values()) await plugin.start();
  }

  async call(pluginName: string, payload: Buffer): Promise<Buffer> {
    const plugin = this.plugins.get(pluginName);
    if (!plugin) throw new ConfigError(`unknown plugin ${pluginName}`);
    return plugin.call(payload);
  }

  async shutdown(): Promise<void> {
    await Promise.all([...this.plugins.values()].map((plugin) => plugin.shutdown()));
  }
}
