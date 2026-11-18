package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// This covers the observed insert only. The capture also contains separate
// confirmation and external-message controls, which this test does not claim.
func TestStatusAutomationInsertNative(t *testing.T) {
	const fixture = "testdata/native_conformance/status_automation_insert"
	data, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	anonymous, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, mode := range []string{"off", "on"} {
			t.Run(api+"/"+mode, func(t *testing.T) {
				want := map[string]string{}
				for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
					cols := strings.Split(line, "\t")
					if len(cols) >= 4 && cols[0] == api && cols[1] == mode && (cols[2] == "exact_pre" || cols[2] == "exact_insert") {
						if _, exists := want[cols[2]]; exists {
							t.Fatalf("duplicate native row: %s", cols[2])
						}
						want[cols[2]] = strings.Join(cols[3:], "|")
					}
				}
				if len(want) != 2 {
					t.Fatalf("missing native observations: %v", want)
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				err := filepath.WalkDir(filepath.Join(fixture, "force-app"), func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					name, err := filepath.Rel(fixture, path)
					if err != nil {
						return err
					}
					if mode == "off" && strings.Contains(filepath.ToSlash(name), "/flows/") {
						return nil
					}
					source, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if strings.Contains(filepath.ToSlash(name), "/flows/") {
						source = []byte(strings.ReplaceAll(string(source), "<apiVersion>67.0</apiVersion>", "<apiVersion>"+api+"</apiVersion>"))
					}
					writeFile(t, filepath.Join(root, name), string(source))
					return nil
				})
				if err != nil {
					t.Fatal(err)
				}
				index := loadTestIndex(t, root)
				if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
					t.Fatalf("native metadata rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				program, err := vm.CompileAnonymousWithOptions(string(anonymous), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				machine := runner.newMachine()
				machine.SetAssumeDeployedPermissionSets(true)
				result, err := machine.Execute(program)
				if err != nil {
					t.Fatalf("anonymous execution: %v debug=%v", err, result.Debug)
				}
				got := map[string]string{}
				for _, message := range result.Debug {
					value, ok := strings.CutPrefix(message, "OBSERVED|")
					if !ok {
						continue
					}
					key, value, ok := strings.Cut(value, "|")
					if !ok || got[key] != "" {
						t.Fatalf("invalid observation: %s", message)
					}
					got[key] = value
				}
				for key, expected := range want {
					if got[key] != expected {
						t.Errorf("%s: native %q; Glade %q", key, expected, got[key])
					}
				}
			})
		}
	}
}
