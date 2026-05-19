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
exports.spawnPlugin = spawnPlugin;
exports.echo = echo;
exports.kill = kill;
const child_process = __importStar(require("child_process"));
const os = __importStar(require("os"));
const path = __importStar(require("path"));
const pluginart_wire_1 = require("./pluginart_wire");
async function spawnPlugin(cmd, args, label) {
    const sockPath = path.join(os.tmpdir(), `pluginart-${label}-${process.pid}.sock`);
    const proc = child_process.spawn(cmd, args, {
        env: { ...process.env, PLUGIN_SOCKET: sockPath },
        stdio: ['ignore', 'pipe', 'inherit'],
    });
    await waitReady(proc, label);
    const { sock, reader } = await (0, pluginart_wire_1.connectTo)({ sockPath }, label);
    return { sock, reader, proc };
}
function waitReady(proc, label) {
    return new Promise((resolve, reject) => {
        proc.once('error', reject);
        proc.once('exit', (code) => reject(new Error(`${label} exited early with code ${code}`)));
        let buf = '';
        proc.stdout.on('data', (chunk) => {
            buf += chunk.toString();
            if (buf.includes('READY'))
                resolve();
        });
    });
}
async function echo(handle, requestBytes) {
    return (0, pluginart_wire_1.callPlugin)(handle.sock, handle.reader, requestBytes);
}
function kill(handle) {
    handle.sock.destroy();
    handle.proc.kill();
}
