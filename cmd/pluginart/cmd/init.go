package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialise pluginart scaffolding",
}

var initSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Generate a FlatBuffers schema boilerplate for a new plugin",
	RunE:  runInitSchema,
}

var (
	initFlagName string
	initFlagOut  string
)

func init() {
	initSchemaCmd.Flags().StringVar(&initFlagName, "name", "myplugin", "plugin namespace name")
	initSchemaCmd.Flags().StringVar(&initFlagOut, "out", "./schema", "output directory")
	initCmd.AddCommand(initSchemaCmd)
}

func runInitSchema(_ *cobra.Command, _ []string) error {
	if err := os.MkdirAll(initFlagOut, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	tmpl, err := template.New("schema").Parse(schemaTmpl)
	if err != nil {
		return fmt.Errorf("parse schema template: %w", err)
	}

	schemaPath := filepath.Join(initFlagOut, initFlagName+".fbs")
	f, err := os.Create(schemaPath)
	if err != nil {
		return fmt.Errorf("create schema file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := tmpl.Execute(f, struct{ Name string }{Name: initFlagName}); err != nil {
		return fmt.Errorf("render schema template: %w", err)
	}

	readmePath := filepath.Join(initFlagOut, "README.md")
	readmeContent := fmt.Sprintf("Edit `%s.fbs` to define your plugin methods, then run:\n\n"+
		"    pluginart gen client --lang go --schema ./%s/%s.fbs\n"+
		"    pluginart gen plugin --lang go --name %s --schema ./%s/%s.fbs\n",
		initFlagName, initFlagOut, initFlagName, initFlagName, initFlagOut, initFlagName)
	if err := os.WriteFile(readmePath, []byte(readmeContent), 0o644); err != nil {
		return fmt.Errorf("write README: %w", err)
	}

	fmt.Printf("✓ Schema written to %s\n", schemaPath)
	return nil
}
