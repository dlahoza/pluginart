package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"

	"example-plugin/echo"

	"github.com/dlahoza/pluginart/pkg/protocol"
	flatbuffers "github.com/google/flatbuffers/go"
)

const contractHash = "sha256:094f99745014e0e307ad2b73394a45887059d3ce7fcda59fbd741f77e7904a14"

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

	server := protocol.NewServer(ln, &EchoHandler{}, contractHash)
	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}

type EchoHandler struct{}

func (h *EchoHandler) Handle(_ context.Context, payload []byte) ([]byte, error) {
	echoReq, call, err := echo.DecodeEchoRequest(payload)
	if err != nil {
		return nil, err
	}
	output := strings.ToUpper(string(echoReq.Input()))

	b := flatbuffers.NewBuilder(128)
	outOff := b.CreateString(output)
	echo.EchoResponseStart(b)
	echo.EchoResponseAddOutput(b, outOff)
	echoRespOff := echo.EchoResponseEnd(b)
	return echo.BuildEchoCallResponse(call, b, echoRespOff), nil
}
