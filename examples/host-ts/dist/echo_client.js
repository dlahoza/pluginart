"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.echoClient = void 0;
class echoClient {
    manager;
    pluginName;
    constructor(manager, pluginName) {
        this.manager = manager;
        this.pluginName = pluginName;
    }
    /** Call Echo on the plugin. payload is a full schema CallRequest FlatBuffer. */
    async Echo(payload) {
        return this.manager.call(this.pluginName, payload);
    }
}
exports.echoClient = echoClient;
