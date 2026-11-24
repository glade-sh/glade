package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRoad547NativeConformance(t *testing.T) {
	const dir = "testdata/conformance/road_547"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]string
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 17 {
		t.Fatalf("incomplete native rows: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		row := row
		if len(row) != 5 || cases[row[2]] == "" {
			t.Fatalf("invalid row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if seen[key] {
			t.Fatalf("duplicate row: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+row[0]+`"}`)
			base := filepath.Join(root, "force-app", "main", "default")
			for _, name := range []string{"objects/RoadProbe__c/RoadProbe__c.object-meta.xml", "objects/RoadConfig__mdt/RoadConfig__mdt.object-meta.xml", "customMetadata/RoadConfig.Sample.md-meta.xml"} {
				b, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(base, name), string(b))
			}
			class := "@IsTest private class Road547Test { @IsTest static void probe() { " + cases[row[2]] + " } }"
			writeFile(t, filepath.Join(base, "classes/Road547Test.cls"), class)
			writeFile(t, filepath.Join(base, "classes/Road547Test.cls-meta.xml"), "<ApexClass><apiVersion>"+row[0]+"</apiVersion><status>Active</status></ApexClass>")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			if row[1] == "anonymous" {
				analysis := sema.AnalyzeAnonymous(index, cases[row[2]], row[0])
				if row[3] == "compile_error" {
					if !analysis.HasErrors() {
						t.Fatal("native compilation rejected the source")
					}
					for _, d := range analysis.Diagnostics {
						if d.Message == row[4] || d.NativeMessage == row[4] {
							return
						}
					}
					t.Fatalf("native=%s diagnostics=%v", row[4], analysis.Diagnostics)
				}
				if analysis.HasErrors() {
					t.Fatalf("native compiled; diagnostics=%v", analysis.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(cases[row[2]], vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = row[0]
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, line := range result.Debug {
					if strings.Contains(line, "ROAD547|"+row[4]) {
						found = true
					}
				}
				if !found {
					t.Fatalf("native=%q debug=%v", row[4], result.Debug)
				}
				return
			}
			if row[1] != "istest" {
				t.Fatal("unknown route")
			}
			run := Run(index, Options{SelectedClasses: []string{"Road547Test"}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("test runner: %+v", run.Summary())
			}
			problem := run.Suites[0].Cases[0].Problem
			if row[3] == "passed" {
				if problem != nil {
					t.Fatalf("native passed: %+v", problem)
				}
				return
			}
			wantType, wantMessage, ok := strings.Cut(row[4], ": ")
			if !ok || problem == nil || strings.TrimPrefix(problem.Type, "System.") != strings.TrimPrefix(wantType, "System.") || problem.Message != wantMessage {
				t.Fatalf("native=%q, Glade=%+v", row[4], problem)
			}
		})
	}
}

// TestRoad547NeighborBaseline pins the pre-fix anonymous behavior of shapes
// outside the captured empty-constructor insert.
func TestRoad547NeighborBaseline(t *testing.T) {
	cases := []string{
		`insert new Account();`,
		`insert new BatchApexErrorEvent(Message='x');`,
		`BatchApexErrorEvent e = new BatchApexErrorEvent(); insert e;`,
		`delete new BatchApexErrorEvent();`,
		`insert new List<BatchApexErrorEvent>();`,
		`insert new BatchApexErrorEvent();`,
	}
	for _, api := range []string{"61.0", "62.0", "67.0", "68.0"} {
		for _, source := range cases {
			if source == `insert new BatchApexErrorEvent();` && (api == "62.0" || api == "67.0") {
				continue
			}
			result := sema.AnalyzeAnonymous(typesys.Index{}, source, api)
			if len(result.Diagnostics) != 0 {
				t.Errorf("main accepted %s at %s; diagnostics=%v", source, api, result.Diagnostics)
			}
		}
	}
}
