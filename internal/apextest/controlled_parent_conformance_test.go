package apextest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestControlledParentReadNativeConformance(t *testing.T) {
	var data struct {
		APIVersions  []string          `json:"apiVersions"`
		Files        map[string]string `json:"files"`
		Anonymous    string            `json:"anonymous"`
		Observations []struct {
			APIVersion string `json:"apiVersion"`
			Context    string `json:"context"`
			Outcome    string `json:"outcome"`
			Value      string `json:"value"`
		} `json:"observations"`
	}
	raw, err := os.ReadFile("testdata/conformance/controlled_parent_read.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || len(data.Observations) != 6 || len(data.Files) == 0 || data.Anonymous == "" {
		t.Fatal("incomplete captured controlled-parent matrix")
	}
	seen := make(map[string]bool)
	for _, row := range data.Observations {
		key := row.APIVersion + "/" + row.Context
		validAPI := row.APIVersion == "62.0" || row.APIVersion == "67.0" || row.APIVersion == "68.0"
		if !validAPI || (row.Context != "anonymous" && row.Context != "isTest") || seen[key] || row.Outcome != "runtime" || row.Value == "" {
			t.Fatalf("invalid captured cell %s: %#v", key, row)
		}
		seen[key] = true
	}
	for _, api := range data.APIVersions {
		for _, context := range []string{"anonymous", "isTest"} {
			if !seen[api+"/"+context] {
				t.Fatalf("missing captured cell %s/%s", api, context)
			}
		}
	}
	for _, row := range data.Observations {
		t.Run(row.APIVersion+"/"+row.Context, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row.APIVersion+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for path, source := range data.Files {
				path = strings.TrimPrefix(filepath.ToSlash(path), "src/")
				if (!strings.HasPrefix(path, "classes/") && !strings.HasPrefix(path, "objects/")) || strings.Contains(path, "..") {
					t.Fatalf("invalid captured source path %q", path)
				}
				target := filepath.Join(root, "force-app/main/default", filepath.FromSlash(path))
				if strings.HasSuffix(path, ".cls-meta.xml") {
					continue
				}
				writeFile(t, target, source)
				if strings.HasSuffix(path, ".cls") {
					writeFile(t, target+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row.APIVersion+`</apiVersion><status>Active</status></ApexClass>`)
				}
			}
			index := loadTestIndex(t, root)
			var message string
			if row.Context == "isTest" {
				run := Run(index, Options{SelectedClasses: []string{"DetailVisibilityTest"}, SelectedMethod: "observe", Parallelism: 1})
				if run.Summary().Total != 1 {
					t.Fatalf("expected one captured test: %#v", run.Summary())
				}
				for _, suite := range run.Suites {
					for _, result := range suite.Cases {
						if result.Problem == nil {
							t.Fatal("missing captured assertion transport")
						}
						message = result.Problem.Message
					}
				}
			} else {
				org := orgFromIndex(index)
				org.APIVersion = row.APIVersion
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(data.Anonymous, vm.CompileOptions{APIVersion: row.APIVersion})
				if err != nil {
					t.Fatal(err)
				}
				_, execErr := runner.execute(program)
				var fault *vm.RuntimeError
				if !errors.As(execErr, &fault) || fault.Type != "System.AssertException" {
					t.Fatalf("missing captured assertion transport: %v", execErr)
				}
				message = fault.ExceptionMessage()
			}
			_, actual, found := strings.Cut(message, "DETAIL_CELL|")
			if !found || actual != row.Value {
				t.Fatalf("native value=%q; local transport=%q", row.Value, message)
			}
		})
	}
}
