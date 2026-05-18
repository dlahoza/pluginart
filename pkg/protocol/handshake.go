package protocol

import (
	flatbuffers "github.com/google/flatbuffers/go"

	fb "github.com/dlahoza/pluginart/pkg/protocol/fb/pluginart"
)

func buildHandshakeRequest(contractHash, pluginName string) []byte {
	b := flatbuffers.NewBuilder(256)
	ch := b.CreateByteString([]byte(contractHash))
	pn := b.CreateByteString([]byte(pluginName))
	fb.HandshakeRequestStart(b)
	fb.HandshakeRequestAddContractHash(b, ch)
	fb.HandshakeRequestAddPluginName(b, pn)
	fb.HandshakeRequestAddProtocolVersion(b, 1)
	off := fb.HandshakeRequestEnd(b)
	fb.FinishHandshakeRequestBuffer(b, off)
	return b.FinishedBytes()
}

func buildHandshakeResponse(ok bool, errMsg string) []byte {
	b := flatbuffers.NewBuilder(256)
	var errOff flatbuffers.UOffsetT
	if errMsg != "" {
		errOff = b.CreateByteString([]byte(errMsg))
	}
	fb.HandshakeResponseStart(b)
	fb.HandshakeResponseAddOk(b, ok)
	if errMsg != "" {
		fb.HandshakeResponseAddError(b, errOff)
	}
	off := fb.HandshakeResponseEnd(b)
	fb.FinishHandshakeResponseBuffer(b, off)
	return b.FinishedBytes()
}

func buildPing(seq uint64) []byte {
	b := flatbuffers.NewBuilder(256)
	fb.PingStart(b)
	fb.PingAddSeq(b, seq)
	off := fb.PingEnd(b)
	fb.FinishPingBuffer(b, off)
	return b.FinishedBytes()
}

func buildPong(seq uint64) []byte {
	b := flatbuffers.NewBuilder(256)
	fb.PongStart(b)
	fb.PongAddSeq(b, seq)
	off := fb.PongEnd(b)
	fb.FinishPongBuffer(b, off)
	return b.FinishedBytes()
}

func buildPluginError(code uint16, message string, retry bool) []byte {
	b := flatbuffers.NewBuilder(256)
	msgOff := b.CreateByteString([]byte(message))
	fb.PluginErrorStart(b)
	fb.PluginErrorAddCode(b, code)
	fb.PluginErrorAddMessage(b, msgOff)
	fb.PluginErrorAddRetry(b, retry)
	off := fb.PluginErrorEnd(b)
	fb.FinishPluginErrorBuffer(b, off)
	return b.FinishedBytes()
}
