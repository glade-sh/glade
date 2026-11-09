package apextest

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestQ3ChildSemiJoinNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/q3semijoin"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if row[2] != "seeded_child_in_and_not_in" {
			continue
		}
		api, route := row[0], row[1]
		key := api + "/" + route
		if seen[key] || row[3] != "compiled and passed: one matching and one nonmatching contact" {
			t.Fatalf("invalid capture: %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			class := "Q3Synthetic" + strings.TrimSuffix(api, ".0")
			for _, suffix := range []string{".cls", ".cls-meta.xml"} {
				data, err := os.ReadFile(filepath.Join(fixture, "project/classes", class+suffix))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", class+suffix), string(data))
			}
			for _, suffix := range []string{".page", ".page-meta.xml"} {
				data, err := os.ReadFile(filepath.Join(fixture, "project/pages", "Q3MixedCase"+suffix))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/pages", "Q3MixedCase"+suffix), string(data))
			}
			index := loadTestIndex(t, root)
			if route == "istest" {
				run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: "child", NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("named result: %+v; %s", s, firstRunProblem(run))
				}
				return
			}
			if route != "anonymous" {
				t.Fatalf("unexpected route %s", route)
			}
			body, err := os.ReadFile(filepath.Join(fixture, "seeded.apex"))
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err := runner.newMachine().Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(seen) != 4 {
		t.Fatalf("missing native route/version rows: %v", seen)
	}
}

func TestQ3BundleNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/q3semijoin"
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		if row[2] != "bundle" {
			continue
		}
		api, route := row[0], row[1]
		key := api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate observation %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			version := strings.TrimSuffix(api, ".0")
			testClass := "Q3Synthetic" + version
			helper := "Q3Bundle" + version
			for _, item := range []struct{ dir, name string }{{"project/classes", testClass}, {"bundleclasses", helper}} {
				for _, ext := range []string{".cls", ".cls-meta.xml"} {
					b, err := os.ReadFile(filepath.Join(fixture, item.dir, item.name+ext))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/classes", item.name+ext), string(b))
				}
			}
			for _, ext := range []string{".page", ".page-meta.xml"} {
				b, err := os.ReadFile(filepath.Join(fixture, "project/pages", "Q3MixedCase"+ext))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/pages", "Q3MixedCase"+ext), string(b))
			}
			index := loadTestIndex(t, root)
			if route == "istest" {
				run := Run(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: "bundle", NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("named result: %+v; %s", s, firstRunProblem(run))
				}
				return
			}
			if route != "anonymous" {
				t.Fatalf("unknown route %s", route)
			}
			body, err := os.ReadFile(filepath.Join(fixture, "bundle"+version+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			machine := runner.newMachine()
			machine.RegisterPageReference("Q3MixedCase")
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(seen) != 4 {
		t.Fatalf("missing native rows: %v", seen)
	}
}
