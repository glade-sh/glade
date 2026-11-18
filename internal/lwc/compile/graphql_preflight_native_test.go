package compile_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Replay the existing GraphQL compile rows against the checkout compiler.
func TestGraphQLPreflightNativeCompileRows(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GLADE_ROOT", filepath.Clean(filepath.Join(cwd, "..", "..", "..")))
	data, err := os.ReadFile("testdata/l12_salesforce.json")
	if err != nil {
		t.Fatal(err)
	}
	var all []l12Case
	if err := json.Unmarshal(data, &all); err != nil {
		t.Fatal(err)
	}
	var cases []l12Case
	for _, c := range all {
		if c.Kind == "compile" {
			cases = append(cases, c)
		}
	}
	if len(cases) != 42 {
		t.Fatalf("native GraphQL compile rows = %d, want 42", len(cases))
	}
	for _, api := range []string{"59.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			compiled := l12CompileCases(t, api, cases)
			for _, c := range cases {
				result := compiled[c.ID]
				got := "COMPILE_OK"
				if result.Err != nil {
					got = "COMPILE_ERROR"
				}
				if got != c.Expected[api] {
					t.Errorf("%s: got %s, native %s: %v", c.ID, got, c.Expected[api], result.Err)
				}
				want := c.DiagnosticExpected[api]
				if want == "" {
					continue
				}
				diagnostic := "MISSING_DIAGNOSTIC"
				if len(result.Diagnostics) == 1 {
					diagnostic = result.Diagnostics[0].Message
				} else if result.Err != nil {
					cause := result.Err
					for errors.Unwrap(cause) != nil {
						cause = errors.Unwrap(cause)
					}
					diagnostic = cause.Error()
				}
				if result.Err != nil {
					const prefix = "lwc compile: exit status 1\nError: "
					if strings.HasPrefix(result.Err.Error(), prefix) {
						deployment := strings.TrimSuffix(strings.TrimPrefix(result.Err.Error(), prefix), "\n")
						if strings.HasPrefix(deployment, "[Line: ") && !strings.Contains(deployment, "\n    at ") {
							diagnostic = deployment
						}
					}
				}
				diagnostic, want = l12DiagnosticText(diagnostic), l12DiagnosticText(want)
				if diagnostic != want && l12WireDiagnosticCarries[c.ID] == "" {
					t.Errorf("%s: diagnostic %q, native %q", c.ID, diagnostic, want)
				}
			}
		})
	}
}
