# Config Reference

`pluginart.toml` is read by Go, Python, and TypeScript host runtimes.

```toml
version = 1

[defaults]
startup_timeout = "5s"
shutdown_timeout = "10s"
health_interval = "2s"
max_restarts = 5
restart_backoff_max = "30s"
max_message_bytes = 4194304

[[plugins]]
name = "echo"
type = "binary"
path = "./plugins/echo"
args = []
transport = "unix"
contract_hash = "sha256:<hex>"
env = { LOG_LEVEL = "info" }
```

## Fields

`version` must be `1`.

Defaults: `startup_timeout`, `shutdown_timeout`, `health_interval`, `max_restarts`, `restart_backoff_max`, `max_message_bytes`, and `compression`. Per-plugin values override defaults.

Plugin fields: `name`, `type`, `path`, `args`, `image`, `address`, `transport`, `contract_hash`, `env`, `resources`, `dial_timeout`.

## Modes

`binary` runs `path` with `args`, injects `PLUGIN_SOCKET` by default, or `PLUGIN_ADDR` when `transport = "tcp"`.

`docker` runs `image` with Docker, publishes an allocated TCP port, injects `PLUGIN_ADDR` inside the container, applies `resources.memory` and `resources.cpus`, and waits for `READY` in logs.

`remote` dials `address` over TCP and performs the handshake. Remote plugins are not process-managed.

## Env

`env` is injected verbatim. Plugin configuration is env-only; runtimes do not serialize config files for plugins.
