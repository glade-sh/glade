package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

func TestCartExtensionDispatchNativeConformance(t *testing.T) {
	dir := "../sema/testdata/cart_extension_surface"
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ ID, Name, Source, Anonymous string }
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	sources := map[string]struct{ name, source, anonymous string }{}
	for _, c := range cases {
		sources[c.ID] = struct{ name, source, anonymous string }{c.Name, c.Source, c.Anonymous}
	}
	raw, err = os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	checked, boundaries := 0, 0
	for _, row := range rows[1:] {
		if row[1] == "deploy" {
			continue
		}
		key := strings.Join(row[:3], "/")
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0") || (row[1] != "anonymous" && row[1] != "istest") {
			t.Fatal("invalid native key: " + key)
		}
		seen[key] = true
		if row[3] == "boundary" {
			boundaries++
			t.Logf("native route boundary %s: %s", key, row[4])
			continue
		}
		if row[3] != "true" {
			t.Fatal("unexpected native observation: " + row[3])
		}
		checked++
		c, ok := sources[row[2]]
		if !ok || c.anonymous == "" {
			t.Fatal("missing captured source: " + row[2])
		}
		t.Run(key, func(t *testing.T) {
			files := map[string]string{c.name + ".cls": c.source}
			// Retain only this captured method's wrapper so unrelated extension
			// availability cannot hide the independent dispatch observation.
			if row[1] == "istest" {
				files["GardenSurfaceTest.cls"] = "@IsTest private class GardenSurfaceTest { @IsTest static void " + row[2] + "() { System.assertEquals(true, " + c.name + ".observe()); } }"
			}
			index := nativeBoundaryIndex(t, row[0], files, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native executes; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			if row[1] == "istest" {
				run := Run(index, Options{SelectedClasses: []string{"GardenSurfaceTest"}, SelectedMethod: row[2], NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
					t.Fatalf("expected one captured test: %+v", run.Summary())
				}
				result := run.Suites[0].Cases[0]
				if result.Status != testreport.StatusPass {
					t.Fatalf("native test passed; Glade: %+v", result)
				}
				return
			}
			analysis = sema.AnalyzeAnonymous(index, c.anonymous, row[0])
			if analysis.HasErrors() {
				t.Fatalf("native anonymous compiled; Glade: %v", analysis.Diagnostics)
			}
			program, err := vm.CompileAnonymousWithOptions(c.anonymous, vm.CompileOptions{APIVersion: row[0]})
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
			found := 0
			for _, line := range result.Debug {
				if line == "SURFACE|"+row[3] {
					found++
				}
			}
			if found != 1 {
				t.Fatalf("native SURFACE|%s; Glade debug: %v", row[3], result.Debug)
			}
		})
	}
	if checked != 26 || boundaries != 4 || len(seen) != 30 {
		t.Fatalf("incomplete native runtime matrix: checked=%d boundaries=%d keys=%d", checked, boundaries, len(seen))
	}
}
