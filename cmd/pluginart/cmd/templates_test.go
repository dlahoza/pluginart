package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/dlahoza/pluginart/pkg/schema"
)

func TestGoEnvelopeHelpersTemplate(t *testing.T) {
	data := clientTemplateData{
		Namespace: "echo",
		Methods: []schema.Method{{
			Name:          "Echo",
			RequestTable:  "EchoRequest",
			ResponseTable: "EchoResponse",
		}},
	}

	var out strings.Builder
	tmpl := template.Must(template.New("helpers").Parse(goEnvelopeHelpersTmpl))
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"func BuildEchoCallRequest",
		"RequestPayloadEchoRequest",
		"func DecodeEchoRequest",
		"func BuildEchoCallResponse",
		"ResponsePayloadEchoResponse",
		"func DecodeEchoResponse",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("rendered helpers missing %q:\n%s", want, out.String())
		}
	}
}

func TestGoPluginEnvelopeHelpersTemplate(t *testing.T) {
	data := pluginTemplateData{
		Namespace: "echo",
		Methods: []schema.Method{{
			Name:          "Echo",
			RequestTable:  "EchoRequest",
			ResponseTable: "EchoResponse",
		}},
	}

	var out strings.Builder
	tmpl := template.Must(template.New("plugin_helpers").Parse(goPluginEnvelopeHelpersTmpl))
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"package main",
		"func DecodeEchoRequest",
		"RequestPayloadEchoRequest",
		"func BuildEchoCallResponse",
		"ResponsePayloadEchoResponse",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("rendered plugin helpers missing %q:\n%s", want, out.String())
		}
	}
}

func TestGoPluginBindingsHelpersTemplate(t *testing.T) {
	data := clientTemplateData{
		Namespace: "echo",
		Methods: []schema.Method{{
			Name:          "Echo",
			RequestTable:  "EchoRequest",
			ResponseTable: "EchoResponse",
		}},
	}

	var out strings.Builder
	tmpl := template.Must(template.New("plugin_bindings_helpers").Parse(goPluginBindingsHelpersTmpl))
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"package echo",
		"func DecodeEchoRequest",
		"RequestPayloadEchoRequest",
		"func BuildEchoCallResponse",
		"ResponsePayloadEchoResponse",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("rendered plugin binding helpers missing %q:\n%s", want, out.String())
		}
	}
	for _, notWant := range []string{
		"func BuildEchoCallRequest",
		"func DecodeEchoResponse",
	} {
		if strings.Contains(out.String(), notWant) {
			t.Fatalf("rendered plugin binding helpers unexpectedly contain %q:\n%s", notWant, out.String())
		}
	}
}

func TestGenCommandUsesBindingsNotClient(t *testing.T) {
	var hasBindings, hasClient bool
	for _, command := range genCmd.Commands() {
		switch command.Name() {
		case "bindings":
			hasBindings = true
		case "client":
			hasClient = true
		}
	}
	if !hasBindings {
		t.Fatal("gen command missing bindings subcommand")
	}
	if hasClient {
		t.Fatal("gen command should not register client subcommand")
	}
}

func TestRenderSkeletonFileDoesNotOverwriteByDefault(t *testing.T) {
	oldOverwrite := genPluginFlagOverwriteSkeleton
	genPluginFlagOverwriteSkeleton = false
	t.Cleanup(func() { genPluginFlagOverwriteSkeleton = oldOverwrite })

	path := filepath.Join(t.TempDir(), "handler.py")
	if err := os.WriteFile(path, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := renderSkeletonFile("generated", nil, path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "custom" {
		t.Fatalf("skeleton file was overwritten: %q", got)
	}
}

func TestRenderSkeletonFileOverwritesWithFlag(t *testing.T) {
	oldOverwrite := genPluginFlagOverwriteSkeleton
	genPluginFlagOverwriteSkeleton = true
	t.Cleanup(func() { genPluginFlagOverwriteSkeleton = oldOverwrite })

	path := filepath.Join(t.TempDir(), "handler.py")
	if err := os.WriteFile(path, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := renderSkeletonFile("generated", nil, path); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "generated" {
		t.Fatalf("skeleton file was not overwritten: %q", got)
	}
}

func TestPythonEnvelopeHelpersTemplate(t *testing.T) {
	data := clientTemplateData{
		Namespace: "echo",
		Methods: []schema.Method{{
			Name:          "Echo",
			RequestTable:  "EchoRequest",
			ResponseTable: "EchoResponse",
		}},
	}

	var out strings.Builder
	tmpl := template.Must(template.New("py_helpers").Parse(pyEnvelopeHelpersTmpl))
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"def BuildEchoCallRequest",
		"RequestPayload.RequestPayload.EchoRequest",
		"def DecodeEchoRequest",
		"def BuildEchoCallResponse",
		"ResponsePayload.ResponsePayload.EchoResponse",
		"def DecodeEchoResponse",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("rendered Python helpers missing %q:\n%s", want, out.String())
		}
	}
}

func TestPythonPluginEnvelopeHelpersTemplate(t *testing.T) {
	data := pluginTemplateData{
		Namespace: "echo",
		Methods: []schema.Method{{
			Name:          "Echo",
			RequestTable:  "EchoRequest",
			ResponseTable: "EchoResponse",
		}},
	}

	var out strings.Builder
	tmpl := template.Must(template.New("py_plugin_helpers").Parse(pyPluginEnvelopeHelpersTmpl))
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"from fb.echo import",
		"def DecodeEchoRequest",
		"RequestPayload.RequestPayload.EchoRequest",
		"def BuildEchoCallResponse",
		"ResponsePayload.ResponsePayload.EchoResponse",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("rendered Python plugin helpers missing %q:\n%s", want, out.String())
		}
	}
}

func TestTypeScriptEnvelopeHelpersTemplate(t *testing.T) {
	data := clientTemplateData{
		Namespace: "echo",
		Methods: []schema.Method{{
			Name:          "Echo",
			RequestTable:  "EchoRequest",
			ResponseTable: "EchoResponse",
			RequestFile:   "echo-request",
			ResponseFile:  "echo-response",
		}},
	}

	var out strings.Builder
	tmpl := template.Must(template.New("ts_helpers").Parse(tsEnvelopeHelpersTmpl))
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"export function BuildEchoCallRequest",
		"RequestPayload.EchoRequest",
		"export function DecodeEchoRequest",
		"export function BuildEchoCallResponse",
		"ResponsePayload.EchoResponse",
		"export function DecodeEchoResponse",
		"./echo/echo-request",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("rendered TypeScript helpers missing %q:\n%s", want, out.String())
		}
	}
}

func TestTypeScriptPluginEnvelopeHelpersTemplate(t *testing.T) {
	data := pluginTemplateData{
		Namespace: "echo",
		Methods: []schema.Method{{
			Name:          "Echo",
			RequestTable:  "EchoRequest",
			ResponseTable: "EchoResponse",
			RequestFile:   "echo-request",
			ResponseFile:  "echo-response",
		}},
	}

	var out strings.Builder
	tmpl := template.Must(template.New("ts_plugin_helpers").Parse(tsPluginEnvelopeHelpersTmpl))
	if err := tmpl.Execute(&out, data); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"export function DecodeEchoRequest",
		"RequestPayload.EchoRequest",
		"export function BuildEchoCallResponse",
		"ResponsePayload.EchoResponse",
		"./fb/echo/echo-request",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("rendered TypeScript plugin helpers missing %q:\n%s", want, out.String())
		}
	}
}
