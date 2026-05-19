from __future__ import annotations

import argparse
import gc
import json
import os
import socket
import subprocess
import sys
import tempfile
import time
import tracemalloc
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Callable

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "runtimes" / "python"))

from pluginart import PluginManager
from pluginart.protocol import Client, connect


CONTRACT_HASH = "sha256:pluginart-bench"
PLUGIN_NAME = "bench"
SIZES = (10, 1000, 10000)
HOST_MEMORY_LIMIT = 16 * 1024 * 1024
PLUGIN_MEMORY_LIMIT = 32 * 1024 * 1024


@dataclass
class Result:
    runtime: str
    benchmark: str
    payload_bytes: int
    iterations: int
    ns_per_op: int
    bytes_per_sec: int
    heap_peak_bytes_per_op: int
    heap_retained_bytes_per_op: int


def free_tcp_addr() -> str:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.bind(("127.0.0.1", 0))
        return f"127.0.0.1:{sock.getsockname()[1]}"


def start_python_plugin() -> tuple[str, subprocess.Popen[str]]:
    addr = free_tcp_addr()
    env = os.environ.copy()
    env["PLUGIN_ADDR"] = addr
    env["PYTHONPATH"] = str(ROOT / "runtimes" / "python")
    proc = subprocess.Popen(
        [sys.executable, "bench/python/plugin_server.py"],
        cwd=ROOT,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    assert proc.stdout is not None
    deadline = time.monotonic() + 15
    while time.monotonic() < deadline:
        line = proc.stdout.readline()
        if line.strip() == "READY":
            return addr, proc
        if proc.poll() is not None:
            stderr = proc.stderr.read() if proc.stderr else ""
            raise RuntimeError(f"python benchmark plugin exited before READY: {stderr}")
    proc.kill()
    raise RuntimeError("python benchmark plugin startup timeout")


def stop_process(proc: subprocess.Popen[str]) -> None:
    if proc.poll() is None:
        proc.kill()
    proc.wait(timeout=5)


def protocol_client(addr: str) -> Client:
    host, port = addr.rsplit(":", 1)
    sock = socket.create_connection((host, int(port)), timeout=5)
    return connect(sock, PLUGIN_NAME, CONTRACT_HASH)


def manager_client(addr: str) -> PluginManager:
    cfg = f"""version = 1

[defaults]
startup_timeout = "5s"
shutdown_timeout = "2s"
health_interval = "30s"
max_restarts = 0

[[plugins]]
name = "{PLUGIN_NAME}"
type = "remote"
address = "{addr}"
contract_hash = "{CONTRACT_HASH}"
"""
    tmp = tempfile.NamedTemporaryFile("w", delete=False, suffix=".toml")
    try:
        tmp.write(cfg)
        tmp.close()
        manager = PluginManager.from_config(tmp.name)
        manager.start()
        return manager
    finally:
        os.unlink(tmp.name)


def time_calls(
    benchmark: str,
    payload_size: int,
    iterations: int,
    call: Callable[[bytes], bytes],
) -> Result:
    payload = b"x" * payload_size
    gc.collect()
    tracemalloc.start()
    before_current, _ = tracemalloc.get_traced_memory()
    start = time.perf_counter_ns()
    for _ in range(iterations):
        resp = call(payload)
        if len(resp) != len(payload):
            raise RuntimeError(f"response size {len(resp)} != {len(payload)}")
    elapsed = time.perf_counter_ns() - start
    current, peak = tracemalloc.get_traced_memory()
    tracemalloc.stop()
    ns_per_op = elapsed // iterations
    retained = max(0, current - before_current)
    peak_delta = max(0, peak - before_current)
    return Result(
        runtime="python",
        benchmark=benchmark,
        payload_bytes=payload_size,
        iterations=iterations,
        ns_per_op=ns_per_op,
        bytes_per_sec=int((payload_size * iterations * 1_000_000_000) / elapsed),
        heap_peak_bytes_per_op=peak_delta // iterations,
        heap_retained_bytes_per_op=retained // iterations,
    )


def assert_memory_growth(call: Callable[[bytes], bytes]) -> None:
    payload = b"x" * 10000
    for _ in range(10):
        call(payload)
    gc.collect()
    tracemalloc.start()
    before = tracemalloc.take_snapshot()
    for _ in range(500):
        resp = call(payload)
        if len(resp) != len(payload):
            raise RuntimeError(f"response size {len(resp)} != {len(payload)}")
    gc.collect()
    after = tracemalloc.take_snapshot()
    growth = sum(stat.size_diff for stat in after.compare_to(before, "filename"))
    tracemalloc.stop()
    if growth > HOST_MEMORY_LIMIT:
        raise RuntimeError(f"python retained memory grew by {growth} bytes, want <= {HOST_MEMORY_LIMIT}")


def process_rss(pid: int) -> int | None:
    status = Path(f"/proc/{pid}/status")
    if status.exists():
        for line in status.read_text().splitlines():
            if line.startswith("VmRSS:"):
                return int(line.split()[1]) * 1024
    try:
        out = subprocess.check_output(["ps", "-o", "rss=", "-p", str(pid)], text=True)
        return int(out.strip()) * 1024
    except (OSError, subprocess.CalledProcessError, ValueError):
        return None


def assert_plugin_memory_growth(pid: int, call: Callable[[bytes], bytes]) -> None:
    payload = b"x" * 10000
    for _ in range(10):
        call(payload)
    before = process_rss(pid)
    if before is None:
        return
    for _ in range(500):
        resp = call(payload)
        if len(resp) != len(payload):
            raise RuntimeError(f"response size {len(resp)} != {len(payload)}")
    after = process_rss(pid)
    if after is None:
        return
    growth = after - before
    if growth > PLUGIN_MEMORY_LIMIT:
        raise RuntimeError(f"python plugin RSS grew by {growth} bytes, want <= {PLUGIN_MEMORY_LIMIT}")


def print_table(results: list[Result]) -> None:
    print("| Benchmark | Payload | ns/op | MB/s | Peak heap/op | Retained heap/op |")
    print("| --- | ---: | ---: | ---: | ---: | ---: |")
    for result in results:
        mbps = result.bytes_per_sec / (1024 * 1024)
        print(
            f"| {result.benchmark} | {result.payload_bytes} | {result.ns_per_op} | {mbps:.2f} | "
            f"{result.heap_peak_bytes_per_op} B/op | {result.heap_retained_bytes_per_op} B/op |"
        )


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--iterations", type=int, default=100)
    parser.add_argument("--json", type=Path)
    args = parser.parse_args()

    addr, proc = start_python_plugin()
    results: list[Result] = []
    try:
        client = protocol_client(addr)
        try:
            for size in SIZES:
                results.append(time_calls("protocol_client", size, args.iterations, client.call))
            assert_memory_growth(client.call)
        finally:
            client.close()

        manager = manager_client(addr)
        try:
            for size in SIZES:
                results.append(time_calls("plugin_manager", size, args.iterations, lambda p: manager.call(PLUGIN_NAME, p)))
            assert_memory_growth(lambda p: manager.call(PLUGIN_NAME, p))
        finally:
            manager.shutdown()

        server_client = protocol_client(addr)
        try:
            for size in SIZES:
                results.append(time_calls("plugin_server", size, args.iterations, server_client.call))
            assert_plugin_memory_growth(proc.pid, server_client.call)
        finally:
            server_client.close()
    finally:
        stop_process(proc)

    print_table(results)
    if args.json:
        args.json.parent.mkdir(parents=True, exist_ok=True)
        args.json.write_text(json.dumps([asdict(result) for result in results], indent=2) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
