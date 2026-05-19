package cmd

import (
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
