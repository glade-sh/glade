package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

type capturedScopeRunnerCase struct {
	class, test string
	versions    []string
}

func readScopeRunnerTSV(t *testing.T, path string, header []string) [][]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 || strings.Join(rows[0], "\t") != strings.Join(header, "\t") {
		t.Fatalf("invalid fixture header: %s", path)
	}
	return rows[1:]
}

// Salesforce rows capture metadata deployment and synchronous named @IsTest
// execution. Project runner and memory-cache checks below replay those sources
// locally; they are not separate native project-runner observations.
func TestCapturedScopeFamiliesTestRunner(t *testing.T) {
	const fixture = "testdata/conformance/scope_runner_native"
	cases := map[string]capturedScopeRunnerCase{}
	for _, row := range readScopeRunnerTSV(t, filepath.Join(fixture, "cases.tsv"), []string{"case", "class", "test_class", "api_versions"}) {
		if len(row) != 4 || row[0] == "" || row[1] == "" || cases[row[0]].class != "" {
			t.Fatalf("invalid or duplicate case: %v", row)
		}
		cases[row[0]] = capturedScopeRunnerCase{class: row[1], test: row[2], versions: strings.Split(row[3], ",")}
	}
	if len(cases) != 6 {
		t.Fatalf("expected six captured cases, got %d", len(cases))
	}

	rows := map[string]string{}
	for _, row := range readScopeRunnerTSV(t, filepath.Join(fixture, "org.tsv"), []string{"case", "api", "route", "outcome"}) {
		if len(row) != 4 || cases[row[0]].class == "" || (row[2] != "metadata-deploy" && row[2] != "istest") || strings.Contains(row[3], "CAPTURE_ERROR") {
			t.Fatalf("invalid native row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if _, exists := rows[key]; exists {
			t.Fatalf("duplicate native row: %s", key)
		}
		rows[key] = row[3]
	}
	if len(rows) != 24 {
		t.Fatalf("expected 24 captured observations, got %d", len(rows))
	}

	selected := 0
	for name, captured := range cases {
		for _, api := range captured.versions {
			key := name + "/" + api
			deployment, exists := rows[key+"/metadata-deploy"]
			if !exists {
				t.Fatalf("missing native deployment: %s", key)
			}
			if deployment == "" {
				t.Fatalf("empty native deployment result: %s", key)
			}
			if deployment != "accepted" {
				want := map[string]string{"private_override": "override methods require", "loop_before": "Duplicate variable", "sibling_bad": "Illegal assignment"}[name]
				if want == "" || !strings.Contains(deployment, want) {
					t.Fatalf("unexpected native rejection: %s: %q", key, deployment)
				}
			}
			testOutcome, hasTest := rows[key+"/istest"]
			if hasTest && (deployment != "accepted" || captured.test == "-" || testOutcome != "passed") {
				t.Fatalf("invalid native test observation: %s: %q", key, testOutcome)
			}
			if deployment == "accepted" && captured.test != "-" && !hasTest {
				t.Fatalf("missing native test observation: %s", key)
			}
			selected++
			t.Run(key, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				classes := []string{captured.class}
				if captured.test != "-" {
					classes = append(classes, captured.test)
				}
				for _, class := range classes {
					source, err := os.ReadFile(filepath.Join(fixture, "cases", name, class+".cls"))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, "force-app/main/default/classes", class+".cls")
					writeFile(t, path, string(source))
					writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				rejected := index.HasErrors() || analysis.HasErrors()
				if rejected != (deployment != "accepted") {
					t.Fatalf("native deployment %q; index=%v; semantic diagnostics=%v", deployment, index.Diagnostics, analysis.Diagnostics)
				}
				if rejected {
					wantCodes := map[string][]string{"private_override": {"GLADESEMA016", "GLADESEMA032"}, "loop_before": {"GLADESEMA014"}, "sibling_bad": {"GLADESEMA018"}}[name]
					found := false
					for _, d := range analysis.Diagnostics {
						for _, code := range wantCodes {
							if d.Code == code {
								found = true
							}
						}
					}
					if !found {
						t.Fatalf("native rejected %q; local rejection lacked expected diagnostic: %v", deployment, analysis.Diagnostics)
					}
				}
				if !hasTest {
					return // Salesforce rejected the deployment; no native test ran.
				}
				opts := Options{SelectedClasses: []string{captured.test}, SelectedMethod: "check", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
				for runNumber := 0; runNumber < 2; runNumber++ {
					if runNumber == 1 {
						ResetPerfCounters()
					}
					run := Run(index, opts)
					if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
						t.Fatalf("native test passed; local runner: %+v", run.Summary())
					}
					if runNumber == 1 {
						if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
							t.Fatalf("warm runner did not reuse runtime: %+v", phases)
						}
					}
				}
			})
		}
	}
	if selected != 15 {
		t.Fatalf("expected 15 case/API combinations, got %d", selected)
	}
}
