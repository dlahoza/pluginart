# Schema Guide

A plugin schema is a FlatBuffers file with one namespace, request/response tables, payload unions, and `CallRequest` / `CallResponse` roots.

```fbs
namespace echo;

table EchoRequest { input: string; }
table EchoResponse { output: string; }

union RequestPayload { EchoRequest }
union ResponsePayload { EchoResponse }

table CallRequest {
  request_id: uint64;
  payload: RequestPayload;
}

table CallResponse {
  request_id: uint64;
  payload: ResponsePayload;
}

root_type CallRequest;
```

Each `RequestPayload` union member becomes a method by removing the `Request` suffix. `EchoRequest` becomes `Echo`.

The contract hash is `sha256:` plus the SHA-256 of the raw schema file bytes. Hosts send it during handshake; plugins compare it to their generated `CONTRACT_HASH` or `ContractHash`.

Generated Go clients hide the pluginart RPC envelope with method-level helpers, but developers still build the method payload table with FlatBuffers. Generated Python and TypeScript clients do not build field-level request objects yet. They provide method wrappers over complete schema `CallRequest` bytes.
