package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dlahoza/pluginart/pkg/schema"
	"github.com/spf13/cobra"
)

var genCmd = &cobra.Command{
	Use:   "gen",
	Short: "Generate bindings or plugin code from a schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath())
		}
		return cmd.Help()
	},
}

// ── gen bindings ─────────────────────────────────────────────────────────────

var genBindingsCmd = &cobra.Command{
	Use:   "bindings",
	Short: "Generate host or plugin bindings from a schema",
	RunE:  runGenBindings,
}

var (
	genBindingsFlagLang   string
	genBindingsFlagTarget string
	genBindingsFlagSchema string
	genBindingsFlagOut    string
)

func init() {
	genBindingsCmd.Flags().StringVar(&genBindingsFlagLang, "lang", "", "target language (go, python, typescript)")
	genBindingsCmd.Flags().StringVar(&genBindingsFlagTarget, "target", "", "binding target (host, plugin)")
	genBindingsCmd.Flags().StringVar(&genBindingsFlagSchema, "schema", "./schema/*.fbs", "path to .fbs schema file")
	genBindingsCmd.Flags().StringVar(&genBindingsFlagOut, "out", "./gen/go", "output directory")
	_ = genBindingsCmd.MarkFlagRequired("lang")
	_ = genBindingsCmd.MarkFlagRequired("target")
	genCmd.AddCommand(genBindingsCmd)
}

type clientTemplateData struct {
	Namespace    string
	Methods      []schema.Method
	ContractHash string
}

func runGenBindings(_ *cobra.Command, _ []string) error {
	schemaPath, err := resolveSchema(genBindingsFlagSchema)
	if err != nil {
		return err
	}

	if err := checkFlatc(); err != nil {
		return err
	}

	contractHash, err := schema.ContractHash(schemaPath)
	if err != nil {
		return fmt.Errorf("contract hash: %w", err)
	}

	parsed, err := schema.Parse(schemaPath)
	if err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}

	switch genBindingsFlagTarget {
	case "host", "plugin":
	default:
		return fmt.Errorf("unsupported target %q (supported: host, plugin)", genBindingsFlagTarget)
	}

	switch genBindingsFlagLang {
	case "go":
		return runGenBindingsGo(schemaPath, parsed, contractHash, genBindingsFlagOut, genBindingsFlagTarget)
	case "python":
		return runGenBindingsPython(schemaPath, parsed, contractHash, genBindingsFlagOut, genBindingsFlagTarget)
	case "typescript":
		return runGenBindingsTypeScript(schemaPath, parsed, contractHash, genBindingsFlagOut, genBindingsFlagTarget)
	default:
		return fmt.Errorf("unsupported language %q (supported: go, python, typescript)", genBindingsFlagLang)
	}
}

func runGenBindingsGo(schemaPath string, parsed *schema.Schema, contractHash, rootOut, target string) error {
	outDir := filepath.Join(rootOut, parsed.Namespace)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := runFlatc(schemaPath, rootOut); err != nil {
		return fmt.Errorf("flatc: %w", err)
	}

	data := clientTemplateData{
		Namespace:    parsed.Namespace,
		Methods:      parsed.Methods,
		ContractHash: contractHash,
	}

	if target == "host" {
		if err := renderToFile(clientTmpl, data, filepath.Join(outDir, parsed.Namespace+"_client.go")); err != nil {
			return err
		}
		if err := renderToFile(goEnvelopeHelpersTmpl, data, filepath.Join(outDir, "pluginart_helpers.go")); err != nil {
			return err
		}
	} else {
		if err := renderToFile(goPluginBindingsHelpersTmpl, data, filepath.Join(outDir, "pluginart_helpers.go")); err != nil {
			return err
		}
	}

	contractData := struct{ Package, Hash string }{Package: parsed.Namespace, Hash: contractHash}
	if err := renderToFile(contractTmpl, contractData, filepath.Join(outDir, "contract.go")); err != nil {
		return err
	}

	fmt.Printf("✓ %s Go bindings written to %s/\n", target, outDir)
	return nil
}

// ── gen plugin ───────────────────────────────────────────────────────────────

var genPluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Generate a plugin skeleton",
	RunE:  runGenPlugin,
}

var (
	genPluginFlagLang              string
	genPluginFlagName              string
	genPluginFlagSchema            string
	genPluginFlagOut               string
	genPluginFlagOverwriteSkeleton bool
)

func init() {
	genPluginCmd.Flags().StringVar(&genPluginFlagLang, "lang", "", "target language (go, python, typescript)")
	genPluginCmd.Flags().StringVar(&genPluginFlagName, "name", "", "plugin name")
	genPluginCmd.Flags().StringVar(&genPluginFlagSchema, "schema", "./schema/*.fbs", "path to .fbs schema file")
	genPluginCmd.Flags().StringVar(&genPluginFlagOut, "out", "", "output directory (default ./<name>-plugin)")
	genPluginCmd.Flags().BoolVar(&genPluginFlagOverwriteSkeleton, "overwrite-skeleton", false, "overwrite editable skeleton files if they already exist")
	_ = genPluginCmd.MarkFlagRequired("lang")
	_ = genPluginCmd.MarkFlagRequired("name")
	genCmd.AddCommand(genPluginCmd)
}

type pluginTemplateData struct {
	Name         string
	Namespace    string
	Methods      []schema.Method
	ContractHash string
}

func runGenPlugin(_ *cobra.Command, _ []string) error {
	schemaPath, err := resolveSchema(genPluginFlagSchema)
	if err != nil {
		return err
	}

	if err := checkFlatc(); err != nil {
		return err
	}

	contractHash, err := schema.ContractHash(schemaPath)
	if err != nil {
		return fmt.Errorf("contract hash: %w", err)
	}

	parsed, err := schema.Parse(schemaPath)
	if err != nil {
		return fmt.Errorf("parse schema: %w", err)
	}

	switch genPluginFlagLang {
	case "go":
		return runGenPluginGo(schemaPath, parsed, contractHash)
	case "python":
		return runGenPluginPython(schemaPath, parsed, contractHash)
	case "typescript":
		return runGenPluginTypeScript(schemaPath, parsed, contractHash)
	default:
		return fmt.Errorf("unsupported language %q (supported: go, python, typescript)", genPluginFlagLang)
	}
}

func runGenPluginGo(schemaPath string, parsed *schema.Schema, contractHash string) error {
	outDir := genPluginFlagOut
	if outDir == "" {
		outDir = "./" + genPluginFlagName + "-plugin"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := runGenBindingsGo(schemaPath, parsed, contractHash, filepath.Join(outDir, "plugin"), "plugin"); err != nil {
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
		{pluginMainTmpl, "main.go"},
		{pluginHandlerTmpl, "plugin.go"},
		{pluginGomodTmpl, "go.mod"},
		{pluginDockerfileTmpl, "Dockerfile"},
	}
	for _, f := range files {
		if err := renderSkeletonFile(f.tmpl, data, filepath.Join(outDir, f.name)); err != nil {
			return err
		}
	}

	fmt.Printf("✓ Plugin skeleton written to %s/\n", outDir)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func resolveSchema(pattern string) (string, error) {
	if strings.Contains(pattern, "*") {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return "", fmt.Errorf("glob schema: %w", err)
		}
		if len(matches) == 0 {
			return "", fmt.Errorf("no schema files match %q", pattern)
		}
		return matches[0], nil
	}
	return pattern, nil
}

func checkFlatc() error {
	if _, err := exec.LookPath("flatc"); err != nil {
		return fmt.Errorf("flatc not found on PATH — install FlatBuffers: https://github.com/google/flatbuffers/releases")
	}
	return nil
}

func runFlatc(schemaPath, outDir string) error {
	cmd := exec.Command("flatc", "--go", "--gen-mutable", "-o", outDir, schemaPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func renderToFile(tmplStr string, data any, path string) error {
	tmpl, err := template.New("").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("parse template for %s: %w", filepath.Base(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("render %s: %w", filepath.Base(path), err)
	}
	return nil
}

func renderSkeletonFile(tmplStr string, data any, path string) error {
	if !genPluginFlagOverwriteSkeleton {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return renderToFile(tmplStr, data, path)
}
