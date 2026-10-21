package compile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The unversioned metadata and CurrentPageReference controls were captured on
// the same Salesforce release at API 59, 67, and 68. Compare diagnostic bytes,
// including the native filename and source frame, without normalization.
func TestL01API68AdmissionSalesforceConformance(t *testing.T) {
	requireLWCCompilerToolchain(t)
	data, err := os.ReadFile("testdata/l01_api68_admission.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		l01Case
		Diagnostics map[string][]string `json:"diagnostics"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 6 {
		t.Fatalf("expected six native admission controls, got %d", len(cases))
	}
	for _, api := range []string{"59.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			results := compileConformanceBatch(t, api, len(cases), func(root string, index int) {
				c := cases[index].l01Case
				l01WriteBundle(t, root, c, api)
				// Each native control was deployed separately with this identity.
				old := filepath.Join(root, "force-app", "main", "default", "lwc", "familyCompile")
				bundle := filepath.Join(filepath.Dir(old), "familyL01000")
				if err := os.Rename(old, bundle); err != nil {
					t.Fatal(err)
				}
				for _, suffix := range []string{".js", ".html", ".js-meta.xml"} {
					if err := os.Rename(filepath.Join(bundle, "familyCompile"+suffix), filepath.Join(bundle, "familyL01000"+suffix)); err != nil {
						t.Fatal(err)
					}
				}
				writeCompileFixtureFile(t, filepath.Join(bundle, "familyL01000.js"), strings.ReplaceAll(c.JS, "FamilyCompile", "FamilyL01000"))
			})
			for index, c := range cases {
				t.Run(c.ID, func(t *testing.T) {
					want, observed := c.Expected[api]
					messages, diagnosticsObserved := c.Diagnostics[api]
					if !observed || !diagnosticsObserved || (want != "COMPILE_OK" && want != "COMPILE_ERROR") || (want == "COMPILE_ERROR") != (len(messages) > 0) {
						t.Fatal("incomplete native observation")
					}
					result := results[index]
					got := "COMPILE_OK"
					if result.Err != nil {
						got = "COMPILE_ERROR"
					}
					if got != want {
						t.Errorf("compile result %s, want %s: %v", got, want, result.Err)
					}
					actual := make([]string, 0, len(result.Diagnostics))
					for _, diagnostic := range result.Diagnostics {
						actual = append(actual, diagnostic.Message)
					}
					if result.Err != nil && len(actual) == 0 {
						actual = append(actual, result.Err.Error())
					}
					if !reflect.DeepEqual(actual, messages) {
						t.Errorf("diagnostics %#v, want %#v", actual, messages)
					}
				})
			}
		})
	}
}
