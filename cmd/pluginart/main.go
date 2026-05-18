package main

import (
	"os"

	"github.com/dlahoza/pluginart/cmd/pluginart/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
