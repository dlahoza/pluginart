"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.echoClient = void 0;
const pluginart_wire_1 = require("./pluginart_wire");
class echoClient {
    sock;
    reader;
    constructor(sock, reader) {
        this.sock = sock;
        this.reader = reader;
    }
    static async connect(opts) {
        const { sock, reader } = await (0, pluginart_wire_1.connectTo)(opts, 'echo');
        return new echoClient(sock, reader);
    }
    close() { this.sock.destroy(); }
    /** Call Echo on the plugin. payload is a raw FlatBuffers EchoRequest. */
    async Echo(payload) {
        return (0, pluginart_wire_1.callPlugin)(this.sock, this.reader, payload);
    }
}
exports.echoClient = echoClient;
