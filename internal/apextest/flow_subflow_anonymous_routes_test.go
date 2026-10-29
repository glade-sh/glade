package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Replay the captured test methods as ordinary public methods. These local
// anonymous twins share the existing @IsTest oracle; they are not new captures.
func TestFlowSubflowAnonymousRouteTwins(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			source := filepath.Join("testdata/conformance/flow_subflow_route_twins", "api"+api[:2])
			root := t.TempDir()
			err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if entry.Name() == "native" {
						return filepath.SkipDir
					}
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				text := string(body)
				if filepath.Base(path) == "CedarRouteTest.cls" {
					text = strings.ReplaceAll(text, "@IsTest static void", "public static void")
					text = strings.ReplaceAll(text, "@IsTest", "")
					text = strings.Replace(text, "private class CedarRouteTest", "public class CedarRouteTest", 1)
				}
				writeFile(t, filepath.Join(root, name), text)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			raw, err := os.ReadFile(filepath.Join(source, "native/org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			rows := strings.Split(strings.TrimSpace(string(raw)), "\n")[1:]
			if len(rows) != 11 {
				t.Fatalf("native routes=%d; want 11", len(rows))
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, row := range rows {
				method, outcome, ok := strings.Cut(row, "\t")
				if !ok || outcome != "Pass" {
					t.Fatalf("invalid native row %q", row)
				}
				t.Run(method, func(t *testing.T) {
					invocation := "CedarRouteTest." + method + "();"
					if analysis := sema.AnalyzeAnonymous(index, invocation, api); analysis.HasErrors() {
						t.Fatal(analysis.Diagnostics)
					}
					program, err := vm.CompileAnonymousWithOptions(invocation, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err := runner.newMachine().Execute(program); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}
