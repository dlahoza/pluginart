package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"example-host/echo"

	"github.com/dlahoza/pluginart/pkg/runtime"
	flatbuffers "github.com/google/flatbuffers/go"
)

func main() {
	manager, err := runtime.NewManagerFromConfig("pluginart.toml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		log.Fatalf("start plugins: %v", err)
	}
	defer manager.Shutdown(context.Background())

	payload := buildEchoRequest("hello from host")

	resp, err := manager.Call(context.Background(), "echo", payload)
	if err != nil {
		log.Fatalf("call echo: %v", err)
	}
	fmt.Printf("echo (go):     %s\n", parseEchoResponse(resp))

	resp, err = manager.Call(context.Background(), "echo-py", payload)
	if err != nil {
		log.Fatalf("call echo-py: %v", err)
	}
	fmt.Printf("echo (python): %s\n", parseEchoResponse(resp))
}

func buildEchoRequest(input string) []byte {
	b := flatbuffers.NewBuilder(128)
	inOff := b.CreateString(input)
	echo.EchoRequestStart(b)
	echo.EchoRequestAddInput(b, inOff)
	echoReqOff := echo.EchoRequestEnd(b)
	echo.CallRequestStart(b)
	echo.CallRequestAddPayloadType(b, echo.RequestPayloadEchoRequest)
	echo.CallRequestAddPayload(b, echoReqOff)
	reqOff := echo.CallRequestEnd(b)
	b.Finish(reqOff)
	return b.FinishedBytes()
}

func parseEchoResponse(buf []byte) string {
	resp := echo.GetRootAsCallResponse(buf, 0)
	var tab flatbuffers.Table
	resp.Payload(&tab)
	var echoResp echo.EchoResponse
	echoResp.Init(tab.Bytes, tab.Pos)
	return string(echoResp.Output())
}
