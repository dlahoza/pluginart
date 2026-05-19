# Pluginart Benchmarks

This suite measures Pluginart runtime overhead without generated schema bindings or plugin business logic. Each benchmark sends raw byte payloads through the Pluginart protocol. The benchmark plugin returns the payload unchanged.

## What Is Measured

| Area | Measurement |
| --- | --- |
| Host manager | `PluginManager` config lookup, serialized call path, framing, and response read |
| Protocol client | direct protocol connect, call, framing, and response read |
| Plugin server | handshake, frame read, handler dispatch, and response write |

Go uses the standard benchmark allocation counters, so its output includes `B/op` and `allocs/op`. Python and TypeScript report `Peak heap/op` and `Retained heap/op` because their standard runtimes do not expose exact allocation counts like Go.

Payload sizes are `10`, `1000`, and `10000` bytes. CI runs every case with `100x`.

## Local Setup

Build the TypeScript runtime and benchmark helpers before running the full suite:

```bash
cd runtimes/typescript
npm install
npm run build
cd ../../bench/typescript
npm install
npm run build
```

Build the Go benchmark plugin used by the Go plugin server benchmark:

```bash
mkdir -p bench/bin
go build -o bench/bin/pluginart-bench-go-plugin ./bench/go/bench_plugin
```

Create a Python virtualenv with Homebrew Python and install the Python runtime:

```bash
/opt/homebrew/bin/python3 -m venv .venv
.venv/bin/python -m pip install -e runtimes/python
```

## Run Benchmarks

Run Go host, Go protocol client, and Go plugin server benchmarks:

```bash
go test -tags bench ./bench/go -run TestBench -bench Benchmark -benchmem -benchtime=100x -count=1
```

Run Python host and protocol client benchmarks:

```bash
.venv/bin/python bench/python/run_bench.py --iterations 100 --json bench-results/python.json
```

Run TypeScript host and protocol client benchmarks:

```bash
node --expose-gc bench/typescript/dist/run-bench.js --iterations 100 --json bench-results/typescript.json
```

## Memory Checks

The benchmark suite includes retained memory checks:

| Area | Limit |
| --- | ---: |
| Go host | 8 MiB |
| Python host | 16 MiB |
| TypeScript host | 16 MiB |
| Plugin server process | 32 MiB RSS |

Timing numbers are informational in CI. Correctness failures and memory growth above these limits fail the job.
