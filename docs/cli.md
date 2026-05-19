# CLI Reference

## `pluginart init schema`

Creates a starter FlatBuffers schema.

```bash
pluginart init schema --name echo
```

Output includes request/response tables, payload unions, `CallRequest`, `CallResponse`, and `PluginError`.

## `pluginart gen bindings`

Generates host-side or plugin-side bindings and FlatBuffers code.

```bash
pluginart gen bindings --target host --lang go --schema schema/echo.fbs --out gen/go
pluginart gen bindings --target host --lang python --schema schema/echo.fbs --out gen/python
pluginart gen bindings --target plugin --lang go --schema schema/echo.fbs --out echo-plugin/plugin
```

Host bindings include generated client wrappers. Plugin bindings include schema code, contract hash, and plugin envelope helpers, but no host client wrappers. The repository examples place generated host code under `examples/host-*/plugins/echo` and generated plugin plumbing under `examples/plugin-*/plugin`.

## `pluginart gen plugin`

Generates a plugin skeleton.

```bash
pluginart gen plugin --lang python --name echo --schema schema/echo.fbs --out echo-plugin-py
```

Skeleton files are written in the plugin project root, while generated plumbing is written under `plugin/`. Existing skeleton files are not overwritten unless `--overwrite-skeleton` is passed.

## `pluginart validate`

Validates the schema and prints the contract hash.

```bash
pluginart validate --schema schema/echo.fbs
```
