package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dlahoza/pluginart/pkg/runtime"
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

	resp, err := manager.Call(context.Background(), "echo", []byte("hello from host"))
	if err != nil {
		log.Fatalf("call echo: %v", err)
	}

	fmt.Printf("echo response: %s\n", resp)
}
