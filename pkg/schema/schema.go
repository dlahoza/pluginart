// Package schema provides utilities for working with pluginart .fbs schema files.
package schema

// Schema holds the parsed contents of a .fbs file needed for code generation.
type Schema struct {
	Namespace string   // e.g. "transform"
	Methods   []Method // one per request/response table pair
}

// Method represents a single RPC method inferred from the schema unions.
type Method struct {
	Name         string // e.g. "Example"
	RequestTable string // e.g. "ExampleRequest"
	ResponseTable string // e.g. "ExampleResponse"
}

// ContractHash returns the SHA-256 hash of the schema file at path,
// formatted as "sha256:<hex>".
func ContractHash(path string) (string, error) {
	return contractHash(path)
}

// Parse parses the .fbs file at path and returns the extracted Schema.
func Parse(path string) (*Schema, error) {
	return parse(path)
}
