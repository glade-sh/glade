package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCapturedInheritedEnclosingStaticAnonymous(t *testing.T) {
	const fixture = "testdata/conformance/enclosing_static_capture"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	source, err := os.ReadFile(filepath.Join(fixture, "inherited.apex"))
	if err != nil {
		t.Fatal(err)
	}
	parent, child, ok := strings.Cut(string(source), "\n")
	if !ok {
		t.Fatal("missing captured class source")
	}
	anon, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[0] != "inherited_runtime" {
			continue
		}
		if row[2] != "anonymous" || row[3] != "passed" || seen[row[1]] {
			t.Fatalf("invalid native row: %v", row)
		}
		api := row[1]
		seen[api] = true
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for name, body := range map[string]string{"Parent": parent, "Probe": child} {
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, body)
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("captured project rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			if analysis := sema.AnalyzeAnonymous(index, string(anon), api); analysis.HasErrors() {
				t.Fatalf("captured anonymous code rejected: %v", analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(string(anon), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			if _, err := newConformanceRunner(t, index, conformanceRunnerOptions{Org: &org, LinkProject: true}).execute(program); err != nil {
				t.Fatalf("captured anonymous execution: %v", err)
			}
		})
	}
	if !seen["62.0"] || !seen["67.0"] || len(seen) != 2 {
		t.Fatalf("incomplete native API matrix: %v", seen)
	}
}

func TestCapturedInheritedEnclosingStaticTestRunner(t *testing.T) {
	const fixture = "testdata/conformance/enclosing_static_istest"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
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
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[0] != "inherited_istest" || row[2] != "istest" || row[3] != "passed" || seen[row[1]] {
			t.Fatalf("invalid native row: %v", row)
		}
		api := row[1]
		seen[api] = true
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for _, name := range []string{"Parent", "Probe", "ProbeCheck"} {
				body, err := os.ReadFile(filepath.Join(fixture, name+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, "force-app/main/default/classes", name+".cls")
				writeFile(t, path, string(body))
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("captured project rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			opts := Options{SelectedClasses: []string{"ProbeCheck"}, SelectedMethod: "check", NoDiskCache: true, Parallelism: 1, PerfCounters: true}
			for runNumber := 0; runNumber < 2; runNumber++ {
				if runNumber == 1 {
					ResetPerfCounters()
				}
				run := Run(index, opts)
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("test runner route: %+v", run.Summary())
				}
				if runNumber == 1 {
					if phases := SnapshotPerfCounters().Phases; phases.MemoryCacheHits != 1 || phases.CacheMisses != 0 {
						t.Fatalf("warm route did not reuse runtime: %+v", phases)
					}
				}
			}
		})
	}
	if !seen["62.0"] || !seen["67.0"] || len(seen) != 2 {
		t.Fatalf("incomplete native API matrix: %v", seen)
	}
}
