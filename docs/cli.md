# CLI Reference

## `pluginart init schema`

Creates a starter FlatBuffers schema.

```bash
pluginart init schema --name echo
```

Output includes request/response tables, payload unions, `CallRequest`, `CallResponse`, and `PluginError`.

## `pluginart gen client`

Generates host-side clients and FlatBuffers code.

```bash
pluginart gen client --lang go --schema schema/echo.fbs --out gen/go/echo
pluginart gen client --lang python --schema schema/echo.fbs --out gen/python
pluginart gen client --lang typescript --schema schema/echo.fbs --out gen/typescript
```

Go output includes FlatBuffers modules, a generated client, `pluginart_helpers.go` envelope helpers, and `contract.go`. Python output includes FlatBuffers Python modules, `<namespace>_client.py`, and `contract.py`. TypeScript output includes FlatBuffers TypeScript modules, `<namespace>_client.ts`, and `contract.ts`.

## `pluginart gen plugin`

Generates a plugin skeleton.

```bash
pluginart gen plugin --lang python --name echo --schema schema/echo.fbs --out echo-plugin-py
```

Python and TypeScript skeletons import runtime package server helpers. They do not emit copied wire-protocol helpers.

## `pluginart validate`

Validates the schema and prints the contract hash.

```bash
pluginart validate --schema schema/echo.fbs
```
