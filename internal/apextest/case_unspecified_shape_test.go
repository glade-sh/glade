package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Local contract, not a native row: a project with no scratch definition has
// an unknown org shape and keeps its earlier behavior. The Division write
// restriction without Case metadata is known only for a scratch org
// (standard_case_field_features_division).
func TestCaseUnspecifiedShapeKeepsEarlierBehavior(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			index := loadTestIndex(t, root)
			assign := "Case c = new Case(); c.Division = null; System.debug('R|assigned');"
			if analysis := sema.AnalyzeAnonymous(index, assign, api); analysis.HasErrors() {
				t.Fatalf("assignment diagnostics: %v", analysis.Diagnostics)
			}
			put := "Case c = new Case(); c.put('Division', null); System.debug('R|put');"
			org := orgFromIndex(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			program, err := vm.CompileAnonymousWithOptions(put, vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			result, runErr := runner.execute(program)
			if runErr != nil {
				t.Fatalf("no-scratch dynamic put must run cleanly: %v", runErr)
			}
			if !strings.Contains(strings.Join(result.Debug, "\n"), "R|put") {
				t.Fatalf("put did not complete: %v", result.Debug)
			}
		})
	}
}

func TestExternalUserCaseSharingUnspecifiedShape(t *testing.T) {
	const base = "testdata/conformance/external_user_case_lead_sharing"
	for _, api := range []string{"62.0", "67.0"} {
		for _, variant := range []string{"omitted", "public_read_write", "public_read_write_transfer"} {
			t.Run(api+"/"+variant, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				copyTree(t, filepath.Join(base, "project", "force-app"), filepath.Join(root, "force-app"), api)
				// With neither a declared scratch shape nor an internal model,
				// the earlier behavior uses the internal default.
				want := "11/11/1,1"
				if variant != "omitted" {
					caseMetadata := filepath.Join("objects", "Case", "Case.object-meta.xml")
					copyTree(t, filepath.Join(base, variant, caseMetadata), filepath.Join(root, "force-app/main/default", caseMetadata), api)
					want = ""
					for _, row := range readNativeTSV(t, filepath.Join(base, variant, "org.tsv")) {
						if row["api"] == api && row["id"] == "PclCcTest" {
							_, observation, _ := strings.Cut(row["outcome"], "Assertion Failed: ")
							want = caseVisibility(observation)
						}
					}
					if want == "" {
						t.Fatalf("missing public Case native row for %s", api)
					}
				}
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"PclCcTest"}, SelectedMethod: "run", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
					t.Fatalf("expected one asserted observation: %+v", run.Summary())
				}
				_, got, _ := strings.Cut(run.Suites[0].Cases[0].Problem.Message, "Assertion Failed: ")
				if visibility := caseVisibility(got); visibility != want {
					t.Errorf("Glade=%q visibility %q, want %q", got, visibility, want)
				}
			})
		}
	}
}
