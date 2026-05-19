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
const pluginart_1 = require("pluginart");
const echo_request_1 = require("./plugins/echo/echo/echo-request");
const echo_client_1 = require("./plugins/echo/echo_client");
function buildEchoPayload(input) {
    const b = new flatbuffers.Builder(256);
    const inputOff = b.createString(input);
    echo_request_1.EchoRequest.startEchoRequest(b);
    echo_request_1.EchoRequest.addInput(b, inputOff);
    const echoReqOff = echo_request_1.EchoRequest.endEchoRequest(b);
    return { builder: b, payload: echoReqOff };
}
function decodeEchoOutput(response) {
    return response.output() ?? '';
}
async function main() {
    process.chdir(path.resolve(__dirname, '..'));
    const manager = await pluginart_1.PluginManager.fromConfig('pluginart.toml');
    try {
        await manager.start();
        const goClient = new echo_client_1.echoClient(manager, 'echo');
        const pyClient = new echo_client_1.echoClient(manager, 'echo-py');
        let request = buildEchoPayload('hello from ts host');
        console.log(`echo (go):     ${decodeEchoOutput(await goClient.Echo(request.builder, request.payload))}`);
        request = buildEchoPayload('hello from ts host');
        console.log(`echo (python): ${decodeEchoOutput(await pyClient.Echo(request.builder, request.payload))}`);
        const tsClient = new echo_client_1.echoClient(manager, 'echo-ts');
        request = buildEchoPayload('hello from ts host');
        console.log(`echo (ts):     ${decodeEchoOutput(await tsClient.Echo(request.builder, request.payload))}`);
    }
    finally {
        await manager.shutdown();
    }
}
main().catch((err) => {
    console.error(err);
    process.exit(1);
});
