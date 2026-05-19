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
