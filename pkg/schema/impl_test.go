package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestContractHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.fbs")
	data := []byte("namespace example;\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ContractHash(path)
	if err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(data)
	want := "sha256:" + hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("hash = %q, want %q", got, want)
	}
}

func TestContractHashReadError(t *testing.T) {
	if _, err := ContractHash(filepath.Join(t.TempDir(), "missing.fbs")); err == nil {
		t.Fatal("expected error")
	}
}

func TestParse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.fbs")
	content := `
namespace acme.transform.v2;

table EchoRequest {}
table EchoResponse {}
table Ignore {}
table BatchRequest {}
table BatchResponse {}

union RequestPayload {
  EchoRequest,
  Ignore,
  BatchRequest
}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}

	want := &Schema{
		Namespace: "v2",
		Methods: []Method{
			{Name: "Echo", RequestTable: "EchoRequest", ResponseTable: "EchoResponse", RequestFile: "echo-request", ResponseFile: "echo-response"},
			{Name: "Batch", RequestTable: "BatchRequest", ResponseTable: "BatchResponse", RequestFile: "batch-request", ResponseFile: "batch-response"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schema = %#v, want %#v", got, want)
	}
}

func TestParseWithoutRequestPayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.fbs")
	if err := os.WriteFile(path, []byte("namespace sample;\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Namespace != "sample" {
		t.Fatalf("namespace = %q", got.Namespace)
	}
	if len(got.Methods) != 0 {
		t.Fatalf("methods = %#v, want none", got.Methods)
	}
}

func TestParseErrors(t *testing.T) {
	t.Run("read error", func(t *testing.T) {
		if _, err := Parse(filepath.Join(t.TempDir(), "missing.fbs")); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing namespace", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.fbs")
		if err := os.WriteFile(path, []byte("table EchoRequest {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(path); err == nil {
			t.Fatal("expected error")
		}
	})
}
