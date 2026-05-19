from __future__ import annotations

import os
import random
import queue
import socket
import string
import subprocess
import tempfile
import threading
import time
import tomllib
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from .errors import ConfigError, StartupError, TransportError
from .protocol import Client, connect

DEFAULT_STARTUP_TIMEOUT = 5.0
DEFAULT_SHUTDOWN_TIMEOUT = 10.0
DEFAULT_HEALTH_INTERVAL = 2.0
DEFAULT_MAX_RESTARTS = 5
DEFAULT_RESTART_BACKOFF_MAX = 30.0
INITIAL_RESTART_BACKOFF = 1.0
HEALTH_PING_TIMEOUT = 2.0


def _duration(value: Any, default: float) -> float:
    if value is None:
        return default
    if isinstance(value, (int, float)):
        return float(value)
    if not isinstance(value, str):
        raise ConfigError(f"invalid duration {value!r}")
    units = {"ms": 0.001, "s": 1.0, "m": 60.0, "h": 3600.0}
    for suffix, mult in units.items():
        if value.endswith(suffix):
            return float(value[: -len(suffix)]) * mult
    return float(value)


def _free_tcp_addr() -> str:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return f"127.0.0.1:{s.getsockname()[1]}"


def _unix_path() -> str:
    root = Path("/tmp/pluginart")
    try:
        root.mkdir(mode=0o700, parents=True, exist_ok=True)
    except OSError:
        root = Path(tempfile.gettempdir()) / "pluginart"
        root.mkdir(mode=0o700, parents=True, exist_ok=True)
    token = "".join(random.choice(string.hexdigits.lower()) for _ in range(32))
    return str(root / f"{token}.sock")


@dataclass
class RuntimeDefaults:
    startup_timeout: float = DEFAULT_STARTUP_TIMEOUT
    shutdown_timeout: float = DEFAULT_SHUTDOWN_TIMEOUT
    health_interval: float = DEFAULT_HEALTH_INTERVAL
    max_restarts: int = DEFAULT_MAX_RESTARTS
    restart_backoff_max: float = DEFAULT_RESTART_BACKOFF_MAX

    @classmethod
    def from_dict(cls, data: dict[str, Any]) -> "RuntimeDefaults":
        return cls(
            startup_timeout=_duration(data.get("startup_timeout"), DEFAULT_STARTUP_TIMEOUT),
            shutdown_timeout=_duration(data.get("shutdown_timeout"), DEFAULT_SHUTDOWN_TIMEOUT),
            health_interval=_duration(data.get("health_interval"), DEFAULT_HEALTH_INTERVAL),
            max_restarts=int(data.get("max_restarts") or DEFAULT_MAX_RESTARTS),
            restart_backoff_max=_duration(data.get("restart_backoff_max"), DEFAULT_RESTART_BACKOFF_MAX),
        )


@dataclass
class Plugin:
    cfg: dict[str, Any]
    defaults: RuntimeDefaults
    client: Client | None = None
    proc: subprocess.Popen[str] | None = None
    container_id: str = ""
    cancel_health: threading.Event = field(default_factory=threading.Event)
    lock: threading.Lock = field(default_factory=threading.Lock)
    restart_count: int = 0
    restart_backoff: float = INITIAL_RESTART_BACKOFF

    @property
    def name(self) -> str:
        return str(self.cfg.get("name", ""))

    @property
    def contract_hash(self) -> str:
        return str(self.cfg.get("contract_hash", ""))

    @property
    def startup_timeout(self) -> float:
        return _duration(self.cfg.get("startup_timeout"), self.defaults.startup_timeout)

    @property
    def shutdown_timeout(self) -> float:
        return _duration(self.cfg.get("shutdown_timeout"), self.defaults.shutdown_timeout)

    @property
    def health_interval(self) -> float:
        return _duration(self.cfg.get("health_interval"), self.defaults.health_interval)

    def start(self) -> None:
        typ = self.cfg.get("type") or "binary"
        if typ == "binary":
            self._start_binary()
        elif typ == "remote":
            self._start_remote()
        elif typ == "docker":
            self._start_docker()
        else:
            raise ConfigError(f"unsupported plugin type {typ!r}")
        self.cancel_health = threading.Event()
        threading.Thread(target=self._health_loop, daemon=True).start()

    def _addr(self) -> tuple[str, str]:
        typ = self.cfg.get("type") or "binary"
        transport = self.cfg.get("transport") or ""
        if transport == "tcp" or typ in ("remote", "docker"):
            return "tcp", str(self.cfg.get("address") or _free_tcp_addr())
        return "unix", _unix_path()

    def _connect(self, kind: str, addr: str, timeout: float | None = None) -> None:
        sock = socket.socket(socket.AF_UNIX if kind == "unix" else socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(timeout or self.startup_timeout)
        try:
            if kind == "unix":
                sock.connect(addr)
            else:
                host, port = addr.rsplit(":", 1)
                sock.connect((host, int(port)))
            client = connect(sock, self.name, self.contract_hash)
        except Exception:
            sock.close()
            raise
        with self.lock:
            self.client = client

    def _start_binary(self) -> None:
        kind, addr = self._addr()
        env = os.environ.copy()
        env.update({str(k): str(v) for k, v in dict(self.cfg.get("env") or {}).items()})
        env["PLUGIN_ADDR" if kind == "tcp" else "PLUGIN_SOCKET"] = addr
        self.proc = subprocess.Popen(
            [str(self.cfg.get("path")), *[str(a) for a in self.cfg.get("args") or []]],
            env=env,
            stdout=subprocess.PIPE,
            stderr=None,
            text=True,
        )
        self._wait_ready(self.proc, self.startup_timeout, self.name)
        self._connect(kind, addr)

    def _start_remote(self) -> None:
        if not self.cfg.get("address"):
            raise ConfigError(f"remote plugin {self.name!r} requires address")
        self._connect("tcp", str(self.cfg["address"]), _duration(self.cfg.get("dial_timeout"), DEFAULT_STARTUP_TIMEOUT))

    def _start_docker(self) -> None:
        addr = _free_tcp_addr()
        args = ["docker", "run", "-d", "--network", "host", "-e", f"PLUGIN_ADDR={addr}"]
        for k, v in dict(self.cfg.get("env") or {}).items():
            args.extend(["-e", f"{k}={v}"])
        resources = dict(self.cfg.get("resources") or {})
        if resources.get("memory"):
            args.extend(["--memory", str(resources["memory"])])
        if resources.get("cpus"):
            args.extend(["--cpus", str(resources["cpus"])])
        args.append(str(self.cfg.get("image")))
        args.extend(str(a) for a in self.cfg.get("args") or [])
        self.container_id = subprocess.check_output(args, text=True).strip()
        logs = subprocess.Popen(["docker", "logs", "-f", self.container_id], stdout=subprocess.PIPE, text=True)
        self._wait_ready(logs, self.startup_timeout, self.name)
        self._connect("tcp", addr)

    def _wait_ready(self, proc: subprocess.Popen[str], timeout: float, name: str) -> None:
        assert proc.stdout is not None
        lines: queue.Queue[str | None] = queue.Queue()

        def read_stdout() -> None:
            for line in proc.stdout:
                lines.put(line)
            lines.put(None)

        threading.Thread(target=read_stdout, daemon=True).start()
        deadline = time.monotonic() + timeout
        while True:
            if proc.poll() is not None:
                raise StartupError(f"plugin {name!r} exited before READY")
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                proc.kill()
                raise StartupError(f"plugin {name!r} startup timeout")
            try:
                line = lines.get(timeout=min(0.1, remaining))
            except queue.Empty:
                continue
            if line is None:
                raise StartupError(f"plugin {name!r} exited before READY")
            if line.strip() == "READY":
                return

    def _health_loop(self) -> None:
        seq = 0
        while not self.cancel_health.wait(self.health_interval):
            with self.lock:
                client = self.client
            if client is None:
                continue
            try:
                client._sock.settimeout(HEALTH_PING_TIMEOUT)
                seq += 1
                client.ping(seq)
                self.restart_backoff = INITIAL_RESTART_BACKOFF
            except Exception:
                if (self.cfg.get("type") or "binary") != "binary":
                    continue
                if self.restart_count >= int(self.cfg.get("max_restarts") or self.defaults.max_restarts):
                    continue
                time.sleep(self.restart_backoff)
                self.restart_backoff = min(self.restart_backoff * 2, self.defaults.restart_backoff_max)
                self.restart_count += 1
                try:
                    self.shutdown()
                    self._start_binary()
                except Exception:
                    continue

    def call(self, payload: bytes) -> bytes:
        with self.lock:
            client = self.client
        if client is None:
            raise TransportError(f"plugin {self.name!r} not connected")
        return client.call(payload)

    def shutdown(self) -> None:
        self.cancel_health.set()
        with self.lock:
            client = self.client
            self.client = None
        if client is not None:
            client.close()
        if self.container_id:
            subprocess.run(["docker", "stop", self.container_id], timeout=self.shutdown_timeout, check=False)
            subprocess.run(["docker", "rm", self.container_id], check=False)
            self.container_id = ""
        if self.proc is not None and self.proc.poll() is None:
            self.proc.terminate()
            try:
                self.proc.wait(timeout=self.shutdown_timeout)
            except subprocess.TimeoutExpired:
                self.proc.kill()
                self.proc.wait()


class PluginManager:
    def __init__(self, plugins: dict[str, Plugin]) -> None:
        self._plugins = plugins

    @classmethod
    def from_config(cls, path: str) -> "PluginManager":
        try:
            with open(path, "rb") as f:
                cfg = tomllib.load(f)
        except OSError as exc:
            raise ConfigError(f"read {path}: {exc}") from exc
        except tomllib.TOMLDecodeError as exc:
            raise ConfigError(f"parse {path}: {exc}") from exc
        if cfg.get("version") != 1:
            raise ConfigError(f"unsupported config version {cfg.get('version')!r}")
        defaults = RuntimeDefaults.from_dict(dict(cfg.get("defaults") or {}))
        plugins: dict[str, Plugin] = {}
        for pcfg in cfg.get("plugins") or []:
            name = str(pcfg.get("name") or "")
            if not name:
                raise ConfigError("plugin name is required")
            if name in plugins:
                raise ConfigError(f"duplicate plugin name {name!r}")
            plugins[name] = Plugin(dict(pcfg), defaults)
        return cls(plugins)

    def start(self, timeout: float | None = None) -> None:
        deadline = time.monotonic() + timeout if timeout else None
        for plugin in self._plugins.values():
            if deadline is not None and time.monotonic() >= deadline:
                raise StartupError("plugin manager startup timeout")
            plugin.start()

    def call(self, plugin_name: str, payload: bytes) -> bytes:
        if plugin_name not in self._plugins:
            raise ConfigError(f"unknown plugin {plugin_name!r}")
        return self._plugins[plugin_name].call(payload)

    def shutdown(self, timeout: float | None = None) -> None:
        deadline = time.monotonic() + timeout if timeout else None
        for plugin in self._plugins.values():
            if deadline is not None and time.monotonic() >= deadline:
                return
            plugin.shutdown()

    def __enter__(self) -> "PluginManager":
        self.start()
        return self

    def __exit__(self, *_: object) -> None:
        self.shutdown()
