package apextest

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Native observations at both APIs are in testdata/conformance/receiver_exception_details/org.tsv.
func TestReceiverExceptionDetailsNative(t *testing.T) {
	const dir = "testdata/conformance/receiver_exception_details/project"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				content := strings.ReplaceAll(strings.ReplaceAll(string(body), "62.0", api), "67.0", api)
				writeFile(t, filepath.Join(root, name), content)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if a := sema.Analyze(index); index.HasErrors() || a.HasErrors() {
				t.Fatalf("diagnostics: %v %v", index.Diagnostics, a.Diagnostics)
			}
			result := Run(index, Options{SelectedClasses: []string{"ProbeReceiverTest"}, NoDiskCache: true, Parallelism: 1})
			if s := result.Summary(); s.Failed != 0 || s.Passed != 1 {
				t.Fatalf("named controls: %+v, suites: %+v", s, result.Suites)
			}
			body := `System.assertEquals('value:ready', ProbeReceiver.observe(true));
System.assertEquals('caught:System.NullPointerException|Attempt to de-reference a null object|5', ProbeReceiver.observe(false));`
			if a := sema.AnalyzeAnonymous(index, body, api); a.HasErrors() {
				t.Fatalf("anonymous diagnostics: %v", a.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(body, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for i := 0; i < 2; i++ {
				if _, err := runner.execute(program); err != nil {
					t.Fatalf("anonymous run %d: %v", i, err)
				}
			}
		})
	}
}

func TestReceiverMissingDependencyNativeDiagnostic(t *testing.T) {
	const dir = "testdata/conformance/receiver_exception_details"
	body, err := os.ReadFile(filepath.Join(dir, "ProbeMissing.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"classes","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeFile(t, filepath.Join(root, "classes", "ProbeMissing.cls"), string(body))
			writeFile(t, filepath.Join(root, "classes", "ProbeMissing.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			for _, d := range analysis.Diagnostics {
				if d.NativeMessage == "Invalid type: MissingReceiverDependency" {
					return
				}
			}
			t.Fatalf("native missing type diagnostic absent: index=%+v analysis=%+v", index.Diagnostics, analysis.Diagnostics)
		})
	}
}
