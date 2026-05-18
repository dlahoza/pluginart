package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/dlahoza/pluginart/pkg/protocol"
)

func main() {
	var (
		ln  net.Listener
		err error
	)
	switch {
	case os.Getenv("PLUGIN_SOCKET") != "":
		ln, err = net.Listen("unix", os.Getenv("PLUGIN_SOCKET"))
	case os.Getenv("PLUGIN_ADDR") != "":
		ln, err = net.Listen("tcp", os.Getenv("PLUGIN_ADDR"))
	default:
		fmt.Fprintln(os.Stderr, "PLUGIN_SOCKET or PLUGIN_ADDR must be set")
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("READY")

	server := protocol.NewServer(ln, &EchoHandler{}, "")
	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}

// EchoHandler implements protocol.Handler. It returns the payload uppercased.
// In a real plugin this would decode a FlatBuffers CallRequest and encode a CallResponse.
type EchoHandler struct{}

func (h *EchoHandler) Handle(_ context.Context, payload []byte) ([]byte, error) {
	return []byte(strings.ToUpper(string(payload))), nil
}
