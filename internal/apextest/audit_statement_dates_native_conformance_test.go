package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestAuditStatementDatesNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/audit_statement_dates"
	read := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	sources := map[string]string{}
	for _, name := range []string{"CedarAuditProbe.cls", "CedarAuditProbeTest.cls", "CedarAuditChild.trigger", "anonymous.apex"} {
		sources[name] = read("sources/" + name)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			project := t.TempDir()
			writeFile(t, filepath.Join(project, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			for name, source := range sources {
				dir, kind := "classes", "ApexClass"
				if strings.HasSuffix(name, ".apex") {
					continue
				}
				if strings.HasSuffix(name, ".trigger") {
					dir, kind = "triggers", "ApexTrigger"
				}
				path := filepath.Join(project, "force-app/main/default", dir, name)
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", fmt.Sprintf("<%s><apiVersion>%s</apiVersion><status>Active</status></%s>", kind, api, kind))
			}
			index := loadTestIndex(t, project)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native accepts declarations: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runtime := CompileProjectRuntimeForRequest(index)
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(route, func(t *testing.T) {
					path := "native/api" + strings.TrimSuffix(api, ".0") + "/" + route + "/"
					capture := read(path + "RUN-FACTS.txt")
					rowCount := 17
					if route == "istest" {
						rowCount = 18
					}
					for _, fact := range []string{"api=" + api, "class_api=" + api, "context=" + route, fmt.Sprintf("rows=%d", rowCount), "native_complete=true", "deployment_success=true"} {
						if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
							t.Fatal("missing native fact: ", fact)
						}
					}
					for name, source := range sources {
						if !strings.Contains(capture, fmt.Sprintf("source_sha256[%s]=%x\n", name, sha256.Sum256([]byte(source)))) {
							t.Fatal("native source changed: ", name)
						}
					}
					lines := strings.Split(strings.TrimSpace(read(path+"org.tsv")), "\n")
					if len(lines) != rowCount+1 || lines[0] != "id\tvalue" {
						t.Fatal("incomplete native rows")
					}
					native := map[string]any{}
					for _, line := range lines[1:] {
						id, payload, ok := strings.Cut(line, "\t")
						var value any
						if !ok || json.Unmarshal([]byte(payload), &value) != nil {
							t.Fatal("invalid native row: ", line)
						}
						if _, duplicate := native[id]; duplicate {
							t.Fatal("duplicate native row: ", id)
						}
						native[id] = value
					}
					var payload string
					if route == "anonymous" {
						source := sources["anonymous.apex"]
						analysis := sema.AnalyzeAnonymous(index, source, api)
						program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
						if err != nil || analysis.HasErrors() {
							t.Fatalf("native anonymous accepted: %v %v", err, analysis.Diagnostics)
						}
						result, err := runner.execute(program)
						if err != nil || len(result.Debug) != 1 {
							t.Fatalf("anonymous transport: %v %v", err, result.Debug)
						}
						var ok bool
						payload, ok = strings.CutPrefix(result.Debug[0], "PROBE|")
						if !ok {
							t.Fatal("missing observation marker")
						}
					} else {
						// Run includes the real setup snapshot and per-method clock reset.
						run := Run(index, Options{SelectedClasses: []string{"CedarAuditProbeTest"}, SelectedMethod: "observeRows", NoDiskCache: true, Parallelism: 1})
						if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
							t.Fatalf("named selection: %#v", run.Summary())
						}
						observed := run.Suites[0].Cases[0]
						if observed.Problem == nil || !strings.HasSuffix(observed.Problem.Type, "AssertException") {
							t.Fatalf("named observation: %#v", observed)
						}
						_, rest, ok := strings.Cut(observed.Problem.Message, "PROBE|")
						if !ok {
							t.Fatal("missing named observation: ", observed.Problem.Message)
						}
						payload = rest
					}
					got := map[string]any{}
					if err := json.Unmarshal([]byte(payload), &got); err != nil || len(got) != len(native) {
						t.Fatalf("invalid observation: %s (%v)", payload, err)
					}
					// Elapsed timing and single-key ties are observations, not promises.
					// In particular, native named captures also fail the newest-row assumption.
					for _, id := range []string{"audit_not_future", "first_fraction_ms", "second_fraction_ms", "update_fraction_ms", "list_dates_equal", "list_id_desc_second", "update_created_preserved", "update_modified_equals_modstamp"} {
						if got[id] != native[id] {
							t.Errorf("%s: native=%v; Glade=%v", id, native[id], got[id])
						}
					}
					if route == "istest" {
						if got["explicit_date_preserved"] != native["explicit_date_preserved"] {
							t.Errorf("explicit date: native=%v; Glade=%v", native["explicit_date_preserved"], got["explicit_date_preserved"])
						}
						if native["newest_is_second"] != false || native["separate_increasing"] != false || native["setup_before_first"] != false {
							t.Fatal("native newest-row counterexample changed")
						}
					}
					t.Logf("native=%v; Glade=%v", native, got)
				})
			}
		})
	}
}
