package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestScratchCurrencySettingsNativeConformance(t *testing.T) {
	runOrgFeatureObservations(t, "org_feature_settings", "FeatureSettings", 8, []string{"multiCurrency", "Account.CurrencyIsoCode", "Opportunity.CurrencyIsoCode"})
}

// The native TSV pins both source versions and both entry routes. Each phase
// retains its scratch definition; CI only replays the owned source locally.
func runOrgFeatureObservations(t *testing.T, family, class string, count int, facets []string) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", family)
	input, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	reader := csv.NewReader(input)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != count+1 || strings.Join(rows[0], ",") != "source_api,phase,route,observation" {
		t.Fatal("incomplete native feature observations")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 {
			t.Fatal("invalid native row")
		}
		name := strings.Join(row[:3], "/")
		if seen[name] || (row[0] != "62.0" && row[0] != "67.0") || (row[2] != "anonymous" && row[2] != "isTest") {
			t.Fatalf("invalid observation: %s", name)
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", row[0]))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			shape, err := os.ReadFile(filepath.Join(dir, "shapes", row[1]+".json"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(shape))
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			var payload string
			const marker = "OBSERVED|capture|"
			if row[2] == "anonymous" {
				source, err := os.ReadFile(filepath.Join(dir, "probe.apex"))
				if err != nil {
					t.Fatal(err)
				}
				if analysis := sema.AnalyzeAnonymous(index, string(source), row[0]); analysis.HasErrors() {
					t.Fatal(analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				runtime, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil)
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = row[0]
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				for _, message := range result.Debug {
					if strings.HasPrefix(message, marker) {
						if payload != "" {
							t.Fatal("duplicate payload")
						}
						payload = strings.TrimPrefix(message, marker)
					}
				}
			} else {
				run := Run(index, Options{SelectedClasses: []string{class + "Test"}, SelectedMethod: "capture", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("missing observer: %+v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
					t.Fatalf("missing assertion transport: %+v", result)
				}
				_, payload, _ = strings.Cut(result.Problem.Message, marker)
			}
			var got, want map[string]any
			if json.Unmarshal([]byte(payload), &got) != nil || json.Unmarshal([]byte(row[3]), &want) != nil {
				t.Fatalf("invalid observation: %s", payload)
			}
			for _, facet := range facets {
				actual, hasActual := got[facet]
				expected, hasExpected := want[facet]
				if !hasExpected || !hasActual || !reflect.DeepEqual(actual, expected) {
					t.Errorf("%s: native=%v; Glade=%v (payload %s)", facet, expected, actual, payload)
				}
			}
			if len(facets) == 0 && !reflect.DeepEqual(got, want) {
				t.Fatal(fmt.Sprintf("native=%s; Glade=%s", row[3], payload))
			}
		})
	}
}
