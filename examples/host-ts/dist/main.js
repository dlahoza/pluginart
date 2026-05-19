"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
const path = __importStar(require("path"));
const flatbuffers = __importStar(require("flatbuffers"));
const plugin_runner_1 = require("./plugin_runner");
const call_request_1 = require("./echo/echo/call-request");
const call_response_1 = require("./echo/echo/call-response");
const echo_request_1 = require("./echo/echo/echo-request");
const echo_response_1 = require("./echo/echo/echo-response");
const request_payload_1 = require("./echo/echo/request-payload");
const DIR = path.resolve(__dirname, '../..');
function buildEchoCallRequest(input) {
    const b = new flatbuffers.Builder(256);
    const inputOff = b.createString(input);
    echo_request_1.EchoRequest.startEchoRequest(b);
    echo_request_1.EchoRequest.addInput(b, inputOff);
    const echoReqOff = echo_request_1.EchoRequest.endEchoRequest(b);
    call_request_1.CallRequest.startCallRequest(b);
    call_request_1.CallRequest.addRequestId(b, BigInt(1));
    call_request_1.CallRequest.addPayloadType(b, request_payload_1.RequestPayload.EchoRequest);
    call_request_1.CallRequest.addPayload(b, echoReqOff);
    const reqOff = call_request_1.CallRequest.endCallRequest(b);
    call_request_1.CallRequest.finishCallRequestBuffer(b, reqOff);
    return Buffer.from(b.asUint8Array());
}
function decodeEchoOutput(respBytes) {
    const bb = new flatbuffers.ByteBuffer(new Uint8Array(respBytes));
    const resp = call_response_1.CallResponse.getRootAsCallResponse(bb);
    const echoResp = resp.payload(new echo_response_1.EchoResponse());
    return echoResp.output() ?? '';
}
async function main() {
    const goPlugin = await (0, plugin_runner_1.spawnPlugin)(path.join(DIR, 'plugin-go', 'plugin-go'), [], 'go');
    const pyPlugin = await (0, plugin_runner_1.spawnPlugin)('python3', [path.join(DIR, 'plugin-py', 'plugin.py')], 'py');
    const requestBytes = buildEchoCallRequest('hello from ts host');
    const goRespBytes = await (0, plugin_runner_1.echo)(goPlugin, requestBytes);
    const goOutput = decodeEchoOutput(goRespBytes);
    const pyRespBytes = await (0, plugin_runner_1.echo)(pyPlugin, requestBytes);
    const pyOutput = decodeEchoOutput(pyRespBytes);
    console.log(`echo (go):     ${goOutput}`);
    console.log(`echo (python): ${pyOutput}`);
    (0, plugin_runner_1.kill)(goPlugin);
    (0, plugin_runner_1.kill)(pyPlugin);
}
main().catch((err) => {
    console.error(err);
    process.exit(1);
});
