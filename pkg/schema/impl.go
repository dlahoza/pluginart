package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	reNamespace      = regexp.MustCompile(`(?m)^\s*namespace\s+([\w.]+)\s*;`)
	reRequestPayload = regexp.MustCompile(`(?s)union\s+RequestPayload\s*\{([^}]*)\}`)
	reUnionMember    = regexp.MustCompile(`\b(\w+)\b`)
)

func contractHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func parse(path string) (*Schema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := string(data)

	nsMatch := reNamespace.FindStringSubmatch(content)
	if nsMatch == nil {
		return nil, fmt.Errorf("no namespace declaration found in %s", path)
	}
	ns := nsMatch[1]
	if idx := strings.LastIndexByte(ns, '.'); idx >= 0 {
		ns = ns[idx+1:]
	}

	methods := []Method{}
	rpMatch := reRequestPayload.FindStringSubmatch(content)
	if rpMatch != nil {
		body := rpMatch[1]
		for _, m := range reUnionMember.FindAllString(body, -1) {
			if strings.HasSuffix(m, "Request") {
				name := strings.TrimSuffix(m, "Request")
				methods = append(methods, Method{
					Name:          name,
					RequestTable:  m,
					ResponseTable: name + "Response",
				})
			}
		}
	}

	return &Schema{Namespace: ns, Methods: methods}, nil
}
