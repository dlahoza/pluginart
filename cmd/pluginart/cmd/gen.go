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
	Short: "Generate client or plugin code from a schema",
}

// ── gen client ───────────────────────────────────────────────────────────────

var genClientCmd = &cobra.Command{
	Use:   "client",
	Short: "Generate host-side client bindings",
	RunE:  runGenClient,
}

var (
	genClientFlagLang   string
	genClientFlagSchema string
	genClientFlagOut    string
)

func init() {
	genClientCmd.Flags().StringVar(&genClientFlagLang, "lang", "", "target language (only 'go' is supported in v0.1)")
	genClientCmd.Flags().StringVar(&genClientFlagSchema, "schema", "./schema/*.fbs", "path to .fbs schema file")
	genClientCmd.Flags().StringVar(&genClientFlagOut, "out", "./gen/go", "output directory")
	_ = genClientCmd.MarkFlagRequired("lang")
	genCmd.AddCommand(genClientCmd)
}

type clientTemplateData struct {
	Namespace    string
	Methods      []schema.Method
	ContractHash string
}

func runGenClient(_ *cobra.Command, _ []string) error {
	if genClientFlagLang != "go" {
		return fmt.Errorf("only 'go' is supported in v0.1")
	}

	schemaPath, err := resolveSchema(genClientFlagSchema)
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

	outDir := filepath.Join(genClientFlagOut, parsed.Namespace)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := runFlatc(schemaPath, genClientFlagOut); err != nil {
		return fmt.Errorf("flatc: %w", err)
	}

	data := clientTemplateData{
		Namespace:    parsed.Namespace,
		Methods:      parsed.Methods,
		ContractHash: contractHash,
	}

	if err := renderToFile(clientTmpl, data, filepath.Join(outDir, parsed.Namespace+"_client.go")); err != nil {
		return err
	}

	contractData := struct{ Package, Hash string }{Package: parsed.Namespace, Hash: contractHash}
	if err := renderToFile(contractTmpl, contractData, filepath.Join(outDir, "contract.go")); err != nil {
		return err
	}

	fmt.Printf("✓ Client written to %s/\n", outDir)
	return nil
}

// ── gen plugin ───────────────────────────────────────────────────────────────

var genPluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Generate a plugin skeleton",
	RunE:  runGenPlugin,
}

var (
	genPluginFlagLang   string
	genPluginFlagName   string
	genPluginFlagSchema string
	genPluginFlagOut    string
)

func init() {
	genPluginCmd.Flags().StringVar(&genPluginFlagLang, "lang", "", "target language (only 'go' is supported in v0.1)")
	genPluginCmd.Flags().StringVar(&genPluginFlagName, "name", "", "plugin name")
	genPluginCmd.Flags().StringVar(&genPluginFlagSchema, "schema", "./schema/*.fbs", "path to .fbs schema file")
	genPluginCmd.Flags().StringVar(&genPluginFlagOut, "out", "", "output directory (default ./<name>-plugin)")
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
	if genPluginFlagLang != "go" {
		return fmt.Errorf("only 'go' is supported in v0.1")
	}

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

	outDir := genPluginFlagOut
	if outDir == "" {
		outDir = "./" + genPluginFlagName + "-plugin"
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	fbOut := filepath.Join(outDir, "flatbuffers.go")
	if err := consolidateFlatbuffers(schemaPath, parsed.Namespace, fbOut); err != nil {
		return fmt.Errorf("generate flatbuffers: %w", err)
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
		if err := renderToFile(f.tmpl, data, filepath.Join(outDir, f.name)); err != nil {
			return err
		}
	}

	contractData := struct{ Package, Hash string }{Package: "main", Hash: contractHash}
	if err := renderToFile(contractTmpl, contractData, filepath.Join(outDir, "contract.go")); err != nil {
		return err
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

func consolidateFlatbuffers(schemaPath, namespace, outFile string) error {
	tmpDir, err := os.MkdirTemp("", "pluginart-flatc-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	if err := runFlatc(schemaPath, tmpDir); err != nil {
		return err
	}

	genDir := filepath.Join(tmpDir, namespace)
	entries, err := os.ReadDir(genDir)
	if err != nil {
		return err
	}

	var buf strings.Builder
	buf.WriteString("// Code generated by flatc via pluginart. DO NOT EDIT.\npackage main\n\n")
	buf.WriteString("import flatbuffers \"github.com/google/flatbuffers/go\"\n\n")

	first := true
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(genDir, e.Name()))
		if err != nil {
			return err
		}
		content := stripGoHeader(string(data))
		if !first {
			buf.WriteString("\n")
		}
		buf.WriteString(content)
		first = false
	}

	return os.WriteFile(outFile, []byte(buf.String()), 0o644)
}

// stripGoHeader removes the leading generated comment block, package declaration,
// and import block from a Go source string, returning just the declarations.
func stripGoHeader(src string) string {
	lines := strings.Split(src, "\n")
	i := 0
	// skip leading comment lines (// ...)
	for i < len(lines) && (strings.HasPrefix(strings.TrimSpace(lines[i]), "//") || strings.TrimSpace(lines[i]) == "") {
		i++
	}
	// skip package line
	if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "package ") {
		i++
	}
	// skip blank lines after package
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	// skip import block
	if i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "import") {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "(") {
			// multi-line import
			i++
			for i < len(lines) && !strings.Contains(lines[i], ")") {
				i++
			}
			i++ // consume closing )
		} else {
			i++ // single-line import
		}
	}
	// skip blank lines after imports
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	return strings.Join(lines[i:], "\n")
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
