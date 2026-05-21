package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestValidateAcceptsPositionalSchema(t *testing.T) {
	oldSchema := validateFlagSchema
	oldConfig := validateFlagConfig
	validateFlagSchema = ""
	validateFlagConfig = ""
	t.Cleanup(func() {
		validateFlagSchema = oldSchema
		validateFlagConfig = oldConfig
	})

	if err := runValidate(&cobra.Command{}, []string{"../../../examples/schema/echo.fbs"}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMissingTargetShowsUsageOnce(t *testing.T) {
	oldSchema := validateFlagSchema
	oldConfig := validateFlagConfig
	validateFlagSchema = ""
	validateFlagConfig = ""
	t.Cleanup(func() {
		validateFlagSchema = oldSchema
		validateFlagConfig = oldConfig
	})

	var rootOut, rootErr, validateOut, validateErr bytes.Buffer
	cmd := &cobra.Command{
		Use:          "pluginart",
		SilenceUsage: false,
		SilenceErrors: true,
	}
	validate := *validateCmd
	validate.SetOut(&validateOut)
	validate.SetErr(&validateErr)
	cmd.SetOut(&rootOut)
	cmd.SetErr(&rootErr)
	cmd.AddCommand(&validate)
	cmd.SetArgs([]string{"validate"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}
	output := rootErr.String() + rootOut.String() + validateErr.String() + validateOut.String()
	if count := strings.Count(output, "Usage:"); count != 1 {
		t.Fatalf("usage count = %d, want 1\n%s", count, output)
	}
}
