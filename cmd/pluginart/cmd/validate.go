package cmd

import (
	"fmt"
	"net"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/dlahoza/pluginart/pkg/runtime"
	"github.com/dlahoza/pluginart/pkg/schema"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [schema.fbs]",
	Short: "Validate a schema or plugin configuration",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runValidate,
}

var (
	validateFlagSchema string
	validateFlagConfig string
)

func init() {
	validateCmd.Flags().StringVar(&validateFlagSchema, "schema", "", "path to .fbs schema file")
	validateCmd.Flags().StringVar(&validateFlagConfig, "config", "", "path to pluginart.toml")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(_ *cobra.Command, args []string) error {
	if len(args) == 1 && validateFlagSchema == "" && validateFlagConfig == "" {
		validateFlagSchema = args[0]
	}

	if validateFlagSchema == "" && validateFlagConfig == "" {
		return fmt.Errorf("one of --schema or --config is required")
	}

	if validateFlagSchema != "" {
		hash, err := schema.ContractHash(validateFlagSchema)
		if err != nil {
			return fmt.Errorf("schema: %w", err)
		}
		fmt.Printf("contract_hash: %s\n", hash)
		return nil
	}

	var cfg runtime.Config
	if _, err := toml.DecodeFile(validateFlagConfig, &cfg); err != nil {
		return fmt.Errorf("config: %w", err)
	}

	failed := false
	for i := range cfg.Plugins {
		p := &cfg.Plugins[i]
		typ := p.Type
		if typ == "" {
			typ = "binary"
		}
		label := fmt.Sprintf("[%-6s]  %-18s", typ, p.Name)

		switch typ {
		case "binary":
			if _, err := os.Stat(p.Path); err != nil {
				fmt.Printf("%s ✗ path not found: %s\n", label, p.Path)
				failed = true
			} else {
				fmt.Printf("%s ✓ path exists\n", label)
			}
		case "docker":
			if p.Image == "" {
				fmt.Printf("%s ✗ image is required\n", label)
				failed = true
			} else {
				fmt.Printf("%s ✓ image set\n", label)
			}
		case "remote":
			if _, _, err := net.SplitHostPort(p.Address); err != nil {
				fmt.Printf("%s ✗ invalid address (expected host:port): %q\n", label, p.Address)
				failed = true
			} else {
				fmt.Printf("%s ✓ address valid\n", label)
			}
		default:
			fmt.Printf("%s ✗ unknown type %q\n", label, typ)
			failed = true
		}
	}

	if failed {
		return fmt.Errorf("one or more plugins failed validation")
	}
	return nil
}
