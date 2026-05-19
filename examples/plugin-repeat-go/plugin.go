package main

import (
	"context"
	"strings"

	repeat "repeat-plugin/plugin/repeat"

	flatbuffers "github.com/google/flatbuffers/go"
)

// PluginHandler implements the plugin logic. Edit this file.
type PluginHandler struct{}

// Handle is called for every incoming request.
// Use Decode<Method>Request and Build<Method>CallResponse helpers to avoid
// hand-writing the pluginart RPC envelope.
func (h *PluginHandler) Handle(ctx context.Context, payload []byte) ([]byte, error) {
	_ = ctx
	req, call, err := repeat.DecodeRepeatRequest(payload)
	if err != nil {
		return nil, err
	}

	output := strings.Repeat(string(req.Input()), int(req.Count()))

	b := flatbuffers.NewBuilder(128)
	outOff := b.CreateString(output)
	repeat.RepeatResponseStart(b)
	repeat.RepeatResponseAddOutput(b, outOff)
	respOff := repeat.RepeatResponseEnd(b)
	return repeat.BuildRepeatCallResponse(call, b, respOff), nil
}
