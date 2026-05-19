// Plugin entrypoint. Edit this file as needed.
package main

import (
	"fmt"
	"net"
	"os"

	"repeat-plugin/plugin/repeat"

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

	handler := &PluginHandler{}
	server := protocol.NewServer(ln, handler, repeat.ContractHash)
	if err := server.Serve(); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		os.Exit(1)
	}
}
