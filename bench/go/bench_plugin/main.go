package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/dlahoza/pluginart/pkg/protocol"
)

const contractHash = "sha256:pluginart-bench"

type echoHandler struct{}

func (echoHandler) Handle(_ context.Context, payload []byte) ([]byte, error) {
	return payload, nil
}

func main() {
	addr := os.Getenv("PLUGIN_ADDR")
	if addr == "" {
		log.Fatal("PLUGIN_ADDR must be set")
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("READY")

	if err := protocol.NewServer(ln, echoHandler{}, contractHash).Serve(); err != nil {
		log.Fatal(err)
	}
}
