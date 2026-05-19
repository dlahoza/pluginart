package echo

import (
	"testing"

	flatbuffers "github.com/google/flatbuffers/go"
)

func TestEchoEnvelopeHelpers(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	inOff := b.CreateString("hello")
	EchoRequestStart(b)
	EchoRequestAddInput(b, inOff)
	echoReq := EchoRequestEnd(b)

	reqPayload := BuildEchoCallRequest(b, echoReq)
	req, call, err := DecodeEchoRequest(reqPayload)
	if err != nil {
		t.Fatal(err)
	}
	if string(req.Input()) != "hello" {
		t.Fatalf("input = %q, want hello", req.Input())
	}

	b = flatbuffers.NewBuilder(128)
	outOff := b.CreateString("HELLO")
	EchoResponseStart(b)
	EchoResponseAddOutput(b, outOff)
	echoResp := EchoResponseEnd(b)

	respPayload := BuildEchoCallResponse(call, b, echoResp)
	resp, respCall, err := DecodeEchoResponse(respPayload)
	if err != nil {
		t.Fatal(err)
	}
	if respCall.RequestID != call.RequestID {
		t.Fatalf("request id = %d, want %d", respCall.RequestID, call.RequestID)
	}
	if string(resp.Output()) != "HELLO" {
		t.Fatalf("output = %q, want HELLO", resp.Output())
	}
}

func TestDecodeEchoRequestRejectsWrongPayloadType(t *testing.T) {
	b := flatbuffers.NewBuilder(128)
	CallRequestStart(b)
	CallRequestAddPayloadType(b, RequestPayloadNONE)
	req := CallRequestEnd(b)
	b.Finish(req)

	if _, _, err := DecodeEchoRequest(b.FinishedBytes()); err == nil {
		t.Fatal("expected payload type error")
	}
}
