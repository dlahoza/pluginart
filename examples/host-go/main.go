package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"example-host/plugins/echo"
	repeat "example-host/plugins/repeat"

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

	client := echo.NewClient(manager, "echo")
	payloadBuilder, echoReq := buildEchoPayload("hello from host")

	resp, err := client.Echo(context.Background(), payloadBuilder, echoReq)
	if err != nil {
		log.Fatalf("call echo: %v", err)
	}
	fmt.Printf("echo (go):     %s\n", resp.Output())

	pyClient := echo.NewClient(manager, "echo-py")
	payloadBuilder, echoReq = buildEchoPayload("hello from host")
	resp, err = pyClient.Echo(context.Background(), payloadBuilder, echoReq)
	if err != nil {
		log.Fatalf("call echo-py: %v", err)
	}
	fmt.Printf("echo (python): %s\n", resp.Output())

	tsClient := echo.NewClient(manager, "echo-ts")
	payloadBuilder, echoReq = buildEchoPayload("hello from host")
	resp, err = tsClient.Echo(context.Background(), payloadBuilder, echoReq)
	if err != nil {
		log.Fatalf("call echo-ts: %v", err)
	}
	fmt.Printf("echo (ts):     %s\n", resp.Output())

	repeatGoClient := repeat.NewClient(manager, "repeat-go")
	repeatBuilder, repeatReq := buildRepeatPayload("ha", 3)
	repeatResp, err := repeatGoClient.Repeat(context.Background(), repeatBuilder, repeatReq)
	if err != nil {
		log.Fatalf("call repeat-go: %v", err)
	}
	fmt.Printf("repeat (go):     %s\n", repeatResp.Output())

	repeatPyClient := repeat.NewClient(manager, "repeat-py")
	repeatBuilder, repeatReq = buildRepeatPayload("ha", 3)
	repeatResp, err = repeatPyClient.Repeat(context.Background(), repeatBuilder, repeatReq)
	if err != nil {
		log.Fatalf("call repeat-py: %v", err)
	}
	fmt.Printf("repeat (python): %s\n", repeatResp.Output())

	repeatTsClient := repeat.NewClient(manager, "repeat-ts")
	repeatBuilder, repeatReq = buildRepeatPayload("ha", 3)
	repeatResp, err = repeatTsClient.Repeat(context.Background(), repeatBuilder, repeatReq)
	if err != nil {
		log.Fatalf("call repeat-ts: %v", err)
	}
	fmt.Printf("repeat (ts):     %s\n", repeatResp.Output())
}

func buildEchoPayload(input string) (*flatbuffers.Builder, flatbuffers.UOffsetT) {
	b := flatbuffers.NewBuilder(128)
	inOff := b.CreateString(input)
	echo.EchoRequestStart(b)
	echo.EchoRequestAddInput(b, inOff)
	echoReqOff := echo.EchoRequestEnd(b)
	return b, echoReqOff
}

func buildRepeatPayload(input string, count uint32) (*flatbuffers.Builder, flatbuffers.UOffsetT) {
	b := flatbuffers.NewBuilder(128)
	inOff := b.CreateString(input)
	repeat.RepeatRequestStart(b)
	repeat.RepeatRequestAddInput(b, inOff)
	repeat.RepeatRequestAddCount(b, count)
	repeatReqOff := repeat.RepeatRequestEnd(b)
	return b, repeatReqOff
}
