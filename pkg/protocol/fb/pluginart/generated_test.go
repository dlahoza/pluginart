package pluginart

import (
	"testing"

	flatbuffers "github.com/google/flatbuffers/go"
)

func TestHandshakeRequestAccessors(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	hash := b.CreateString("hash")
	name := b.CreateString("plugin")
	HandshakeRequestStart(b)
	HandshakeRequestAddContractHash(b, hash)
	HandshakeRequestAddPluginName(b, name)
	HandshakeRequestAddProtocolVersion(b, 2)
	off := HandshakeRequestEnd(b)
	FinishHandshakeRequestBuffer(b, off)

	req := GetRootAsHandshakeRequest(b.FinishedBytes(), 0)
	if string(req.ContractHash()) != "hash" || string(req.PluginName()) != "plugin" || req.ProtocolVersion() != 2 {
		t.Fatalf("request = hash:%q name:%q version:%d", req.ContractHash(), req.PluginName(), req.ProtocolVersion())
	}
	if req.Table().Bytes == nil {
		t.Fatal("expected table bytes")
	}
	if !req.MutateProtocolVersion(3) || req.ProtocolVersion() != 3 {
		t.Fatalf("mutated version = %d", req.ProtocolVersion())
	}
}

func TestHandshakeRequestDefaultsAndSizePrefixed(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	HandshakeRequestStart(b)
	off := HandshakeRequestEnd(b)
	FinishSizePrefixedHandshakeRequestBuffer(b, off)

	req := GetSizePrefixedRootAsHandshakeRequest(b.FinishedBytes(), 0)
	if req.ContractHash() != nil || req.PluginName() != nil || req.ProtocolVersion() != 1 {
		t.Fatalf("defaults = hash:%q name:%q version:%d", req.ContractHash(), req.PluginName(), req.ProtocolVersion())
	}
}

func TestHandshakeResponseAccessors(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	errOff := b.CreateString("no")
	HandshakeResponseStart(b)
	HandshakeResponseAddOk(b, true)
	HandshakeResponseAddError(b, errOff)
	off := HandshakeResponseEnd(b)
	FinishHandshakeResponseBuffer(b, off)

	resp := GetRootAsHandshakeResponse(b.FinishedBytes(), 0)
	if !resp.Ok() || string(resp.Error()) != "no" {
		t.Fatalf("response = ok:%v error:%q", resp.Ok(), resp.Error())
	}
	if resp.Table().Bytes == nil {
		t.Fatal("expected table bytes")
	}
	if !resp.MutateOk(false) || resp.Ok() {
		t.Fatalf("mutated ok = %v", resp.Ok())
	}
}

func TestHandshakeResponseDefaultsAndSizePrefixed(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	HandshakeResponseStart(b)
	off := HandshakeResponseEnd(b)
	FinishSizePrefixedHandshakeResponseBuffer(b, off)

	resp := GetSizePrefixedRootAsHandshakeResponse(b.FinishedBytes(), 0)
	if resp.Ok() || resp.Error() != nil {
		t.Fatalf("defaults = ok:%v error:%q", resp.Ok(), resp.Error())
	}
}

func TestPingAccessors(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	PingStart(b)
	PingAddSeq(b, 9)
	off := PingEnd(b)
	FinishPingBuffer(b, off)

	ping := GetRootAsPing(b.FinishedBytes(), 0)
	if ping.Seq() != 9 {
		t.Fatalf("seq = %d", ping.Seq())
	}
	if ping.Table().Bytes == nil {
		t.Fatal("expected table bytes")
	}
	if !ping.MutateSeq(10) || ping.Seq() != 10 {
		t.Fatalf("mutated seq = %d", ping.Seq())
	}
}

func TestPingDefaultsAndSizePrefixed(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	PingStart(b)
	off := PingEnd(b)
	FinishSizePrefixedPingBuffer(b, off)

	ping := GetSizePrefixedRootAsPing(b.FinishedBytes(), 0)
	if ping.Seq() != 0 {
		t.Fatalf("default seq = %d", ping.Seq())
	}
}

func TestPongAccessors(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	PongStart(b)
	PongAddSeq(b, 11)
	off := PongEnd(b)
	FinishPongBuffer(b, off)

	pong := GetRootAsPong(b.FinishedBytes(), 0)
	if pong.Seq() != 11 {
		t.Fatalf("seq = %d", pong.Seq())
	}
	if pong.Table().Bytes == nil {
		t.Fatal("expected table bytes")
	}
	if !pong.MutateSeq(12) || pong.Seq() != 12 {
		t.Fatalf("mutated seq = %d", pong.Seq())
	}
}

func TestPongDefaultsAndSizePrefixed(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	PongStart(b)
	off := PongEnd(b)
	FinishSizePrefixedPongBuffer(b, off)

	pong := GetSizePrefixedRootAsPong(b.FinishedBytes(), 0)
	if pong.Seq() != 0 {
		t.Fatalf("default seq = %d", pong.Seq())
	}
}

func TestPluginErrorAccessors(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	msg := b.CreateString("failed")
	PluginErrorStart(b)
	PluginErrorAddCode(b, 7)
	PluginErrorAddMessage(b, msg)
	PluginErrorAddRetry(b, true)
	off := PluginErrorEnd(b)
	FinishPluginErrorBuffer(b, off)

	pe := GetRootAsPluginError(b.FinishedBytes(), 0)
	if pe.Code() != 7 || string(pe.Message()) != "failed" || !pe.Retry() {
		t.Fatalf("plugin error = code:%d message:%q retry:%v", pe.Code(), pe.Message(), pe.Retry())
	}
	if pe.Table().Bytes == nil {
		t.Fatal("expected table bytes")
	}
	if !pe.MutateCode(8) || pe.Code() != 8 {
		t.Fatalf("mutated code = %d", pe.Code())
	}
	if !pe.MutateRetry(false) || pe.Retry() {
		t.Fatalf("mutated retry = %v", pe.Retry())
	}
}

func TestPluginErrorDefaultsAndSizePrefixed(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	PluginErrorStart(b)
	off := PluginErrorEnd(b)
	FinishSizePrefixedPluginErrorBuffer(b, off)

	pe := GetSizePrefixedRootAsPluginError(b.FinishedBytes(), 0)
	if pe.Code() != 0 || pe.Message() != nil || pe.Retry() {
		t.Fatalf("defaults = code:%d message:%q retry:%v", pe.Code(), pe.Message(), pe.Retry())
	}
}
