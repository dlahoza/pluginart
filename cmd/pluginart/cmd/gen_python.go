package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dlahoza/pluginart/pkg/schema"
)

func runGenBindingsPython(schemaPath string, parsed *schema.Schema, contractHash, outDir, target string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	fbOutDir := outDir
	if target == "plugin" {
		fbOutDir = filepath.Join(outDir, "fb")
	}
	if err := runFlatcPython(schemaPath, fbOutDir); err != nil {
		return fmt.Errorf("flatc: %w", err)
	}

	data := clientTemplateData{
		Namespace:    parsed.Namespace,
		Methods:      parsed.Methods,
		ContractHash: contractHash,
	}

	if err := renderToFile(pyContractTmpl, data, filepath.Join(outDir, "contract.py")); err != nil {
		return err
	}

	if target == "host" {
		if err := renderToFile(pyEnvelopeHelpersTmpl, data, filepath.Join(outDir, "pluginart_helpers.py")); err != nil {
			return err
		}
		outFile := filepath.Join(outDir, parsed.Namespace+"_client.py")
		if err := renderToFile(pyClientTmpl, data, outFile); err != nil {
			return err
		}
	} else {
		pluginData := pluginTemplateData{
			Name:         genPluginFlagName,
			Namespace:    parsed.Namespace,
			Methods:      parsed.Methods,
			ContractHash: contractHash,
		}
		if err := renderToFile(pyPluginEnvelopeHelpersTmpl, pluginData, filepath.Join(outDir, "pluginart_helpers.py")); err != nil {
			return err
		}
	}

	fmt.Printf("✓ %s Python bindings written to %s/\n", target, outDir)
	return nil
}

func runGenPluginPython(schemaPath string, parsed *schema.Schema, contractHash string) error {
	outDir := genPluginFlagOut
	if outDir == "" {
		outDir = "./" + genPluginFlagName + "-plugin-py"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := runGenBindingsPython(schemaPath, parsed, contractHash, filepath.Join(outDir, "plugin"), "plugin"); err != nil {
		return err
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
		{pyPluginTmpl, "plugin.py"},
		{pyHandlerTmpl, "handler.py"},
		{pyRequirementsTmpl, "requirements.txt"},
		{pyDockerfileTmpl, "Dockerfile"},
	}
	for _, f := range files {
		if err := renderSkeletonFile(f.tmpl, data, filepath.Join(outDir, f.name)); err != nil {
			return err
		}
	}

	fmt.Printf("✓ Python plugin skeleton written to %s/\n", outDir)
	return nil
}

func runFlatcPython(schemaPath, outDir string) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	cmd := exec.Command("flatc", "--python", "-o", outDir, schemaPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
