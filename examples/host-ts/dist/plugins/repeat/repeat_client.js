"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.repeatClient = void 0;
const pluginart_helpers_1 = require("./pluginart_helpers");
class repeatClient {
    manager;
    pluginName;
    constructor(manager, pluginName) {
        this.manager = manager;
        this.pluginName = pluginName;
    }
    /** Call Repeat on the plugin. payload is a RepeatRequest table offset. */
    async Repeat(builder, payload) {
        const raw = await this.manager.call(this.pluginName, (0, pluginart_helpers_1.BuildRepeatCallRequest)(builder, payload));
        return (0, pluginart_helpers_1.DecodeRepeatResponse)(raw).payload;
    }
}
exports.repeatClient = repeatClient;
