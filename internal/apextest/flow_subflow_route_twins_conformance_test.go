package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
)

// Each route uses a complete synthetic Salesforce project: Apex DML or an
// interview starts the parent, metadata supplies the subflow graph, and the
// child copies the Account reached through its Willow record input. Direct
// interviews and before-save lookups are controls for the shared lookup path.
func TestFlowSubflowRouteTwinsConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_subflow_route_twins"
	cases, err := os.ReadFile(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	methods := strings.Split(strings.TrimSpace(string(cases)), "\n")[1:]
	for _, version := range []string{"api62", "api67"} {
		t.Run(version, func(t *testing.T) {
			source := filepath.Join(fixture, version)
			capture, err := os.ReadFile(filepath.Join(source, "native/RUN-FACTS.txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, fact := range []string{"api=" + strings.TrimPrefix(version, "api") + ".0", "context=istest", "deployment_success=true", "native_complete=true", "rows=11"} {
				if !strings.Contains("\n"+string(capture), "\n"+fact+"\n") {
					t.Fatalf("missing native fact %s", fact)
				}
			}
			rejectionRaw, err := os.ReadFile(filepath.Join(source, "native/before-save-subflow-rejection.json"))
			if err != nil {
				t.Fatal(err)
			}
			var rejection struct {
				API       string
				CheckOnly bool
				Errors    []string
			}
			if json.Unmarshal(rejectionRaw, &rejection) != nil || rejection.API != strings.TrimPrefix(version, "api")+".0" || !rejection.CheckOnly || len(rejection.Errors) != 1 || !strings.Contains(rejection.Errors[0], "Subflow") || !strings.Contains(rejection.Errors[0], "Before Save") {
				t.Fatal("missing native before-save subflow rejection")
			}
			t.Log("native before-save subflow eligibility:", rejection.Errors[0])
			raw, err := os.ReadFile(filepath.Join(source, "native/org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{}
			for _, row := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				method, outcome, ok := strings.Cut(row, "\t")
				if !ok || outcome != "Pass" || want[method] != "" {
					t.Fatalf("invalid native row %q", row)
				}
				want[method] = outcome
			}
			if len(want) != len(methods) {
				t.Fatal("incomplete native route matrix")
			}
			root := t.TempDir()
			err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if entry.Name() == "native" || strings.HasPrefix(entry.Name(), ".") {
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
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(name), sha256.Sum256(body))) {
					t.Fatalf("source differs from native capture: %s", name)
				}
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("project diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			flows, err := automation.LoadProject(p)
			if err != nil || len(flows.Diagnostics) != 0 {
				t.Fatalf("flow metadata diagnostics: %v %v", err, flows.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"CedarRouteTest"}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != len(methods) || len(run.Suites) != 1 {
				t.Fatalf("route selection: %+v", run.Summary())
			}
			for _, row := range methods {
				method, route, _ := strings.Cut(row, "\t")
				t.Run(method, func(t *testing.T) {
					t.Log(route)
					if want[method] != "Pass" {
						t.Fatal("missing native route", method)
					}
					for _, result := range run.Suites[0].Cases {
						if result.MethodName == method {
							if result.Problem != nil {
								t.Fatalf("native Pass; Glade %s: %s", result.Problem.Type, result.Problem.Message)
							}
							if result.Status != "pass" {
								t.Fatalf("native Pass; Glade status %s", result.Status)
							}
							return
						}
					}
					t.Fatal("route did not execute", method)
				})
			}
		})
	}
}
