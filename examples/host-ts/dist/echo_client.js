"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.echoClient = void 0;
const pluginart_helpers_1 = require("./pluginart_helpers");
class echoClient {
    manager;
    pluginName;
    constructor(manager, pluginName) {
        this.manager = manager;
        this.pluginName = pluginName;
    }
    /** Call Echo on the plugin. payload is an EchoRequest table offset. */
    async Echo(builder, payload) {
        const raw = await this.manager.call(this.pluginName, (0, pluginart_helpers_1.BuildEchoCallRequest)(builder, payload));
        return (0, pluginart_helpers_1.DecodeEchoResponse)(raw).payload;
    }
}
exports.echoClient = echoClient;
