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
}

func buildEchoPayload(input string) (*flatbuffers.Builder, flatbuffers.UOffsetT) {
	b := flatbuffers.NewBuilder(128)
	inOff := b.CreateString(input)
	echo.EchoRequestStart(b)
	echo.EchoRequestAddInput(b, inOff)
	echoReqOff := echo.EchoRequestEnd(b)
	return b, echoReqOff
}
