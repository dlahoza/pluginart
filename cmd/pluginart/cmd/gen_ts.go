package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dlahoza/pluginart/pkg/schema"
)

func runGenClientTypeScript(schemaPath string, parsed *schema.Schema, contractHash string) error {
	outDir := genClientFlagOut
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := runFlatcTypeScript(schemaPath, outDir); err != nil {
		return fmt.Errorf("flatc: %w", err)
	}

	data := clientTemplateData{
		Namespace:    parsed.Namespace,
		Methods:      parsed.Methods,
		ContractHash: contractHash,
	}

	if err := renderToFile(tsContractTmpl, data, filepath.Join(outDir, "contract.ts")); err != nil {
		return err
	}
	if err := renderToFile(tsEnvelopeHelpersTmpl, data, filepath.Join(outDir, "pluginart_helpers.ts")); err != nil {
		return err
	}

	outFile := filepath.Join(outDir, parsed.Namespace+"_client.ts")
	if err := renderToFile(tsClientTmpl, data, outFile); err != nil {
		return err
	}

	fmt.Printf("✓ TypeScript client written to %s/\n", outDir)
	return nil
}

func runGenPluginTypeScript(schemaPath string, parsed *schema.Schema, contractHash string) error {
	outDir := genPluginFlagOut
	if outDir == "" {
		outDir = "./" + genPluginFlagName + "-plugin-ts"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	fbOutDir := filepath.Join(outDir, "fb")
	if err := runFlatcTypeScript(schemaPath, fbOutDir); err != nil {
		return fmt.Errorf("flatc: %w", err)
	}

	data := pluginTemplateData{
		Name:         genPluginFlagName,
		Namespace:    parsed.Namespace,
		Methods:      parsed.Methods,
		ContractHash: contractHash,
	}

	files := []struct {
		tmpl string
		name string
	}{
		{tsContractTmpl, "contract.ts"},
		{tsPluginEnvelopeHelpersTmpl, "pluginart_helpers.ts"},
		{tsPluginTmpl, "plugin.ts"},
		{tsHandlerTmpl, "handler.ts"},
		{tsPackageJSONTmpl, "package.json"},
		{tsTsconfigTmpl, "tsconfig.json"},
		{tsDockerfileTmpl, "Dockerfile"},
	}
	for _, f := range files {
		if err := renderToFile(f.tmpl, data, filepath.Join(outDir, f.name)); err != nil {
			return err
		}
	}

	fmt.Printf("✓ TypeScript plugin skeleton written to %s/\n", outDir)
	return nil
}

func runFlatcTypeScript(schemaPath, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("flatc", "--ts", "-o", outDir, schemaPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
