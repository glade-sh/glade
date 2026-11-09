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

func TestQ3StandardSharingReasonNative(t *testing.T) {
	const fixture = "testdata/conformance/q3share"
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
		if row[2] != "standard_manual" {
			continue
		}
		api, route := row[0], row[1]
		key := api + "/" + route
		if seen[key] || row[3] != "AccountShare and OpportunityShare Manual each stringify to Manual" {
			t.Fatalf("bad row %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			name := "Q3Share" + strings.TrimSuffix(api, ".0")
			for _, ext := range []string{".cls", ".cls-meta.xml"} {
				b, err := os.ReadFile(filepath.Join(fixture, "project/classes", name+ext))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+ext), string(b))
			}
			index := loadTestIndex(t, root)
			if route == "istest" {
				run := Run(index, Options{SelectedClasses: []string{name}, SelectedMethod: "observed", NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("named result: %+v; %s", s, firstRunProblem(run))
				}
				return
			}
			if route != "anonymous" {
				t.Fatalf("unexpected route %s", route)
			}
			body, err := os.ReadFile(filepath.Join(fixture, "standard.apex"))
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
		t.Fatalf("missing rows: %v", seen)
	}
}

func TestQ3StandardSharingReasonBoundaries(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			index := loadTestIndex(t, root)
			body, err := os.ReadFile("testdata/conformance/q3share/boundary.apex")
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
}

func TestQ3StandardSharingReasonUnknownRejected(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			index := loadTestIndex(t, root)
			body, err := os.ReadFile("testdata/conformance/q3share/reject.apex")
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				return
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err = runner.newMachine().Execute(program); err == nil {
				t.Fatal("unknown sharing reason was accepted")
			}
		})
	}
}

func TestQ3CapturedProjectWarmRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, row := range []struct{ family, prefix, method string }{{"q3semijoin", "Q3Synthetic", "child"}, {"q3share", "Q3Share", "observed"}, {"shadow", "Q3ShadowTest", "observed"}} {
			t.Run(api+"/"+row.family, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				name := row.prefix + strings.TrimSuffix(api, ".0")
				sourceDir := filepath.Join("testdata/conformance", row.family, "project/classes")
				names := []string{name}
				if row.family == "shadow" {
					sourceDir = "testdata/conformance/q3share/shadow/classes"
					names = append(names, "Q3Shadow"+strings.TrimSuffix(api, ".0"))
				}
				for _, className := range names {
					for _, ext := range []string{".cls", ".cls-meta.xml"} {
						b, err := os.ReadFile(filepath.Join(sourceDir, className+ext))
						if err != nil {
							t.Fatal(err)
						}
						writeFile(t, filepath.Join(root, "force-app/main/default/classes", className+ext), string(b))
					}
				}
				if row.family == "q3semijoin" {
					for _, ext := range []string{".page", ".page-meta.xml"} {
						b, err := os.ReadFile(filepath.Join("testdata/conformance/q3semijoin/project/pages", "Q3MixedCase"+ext))
						if err != nil {
							t.Fatal(err)
						}
						writeFile(t, filepath.Join(root, "force-app/main/default/pages", "Q3MixedCase"+ext), string(b))
					}
				}
				index := loadTestIndex(t, root)
				opts := Options{SelectedClasses: []string{name}, SelectedMethod: row.method, Parallelism: 1, NoDiskCache: true, PerfCounters: true}
				for runNumber := 0; runNumber < 2; runNumber++ {
					if runNumber == 1 {
						ResetPerfCounters()
					}
					run := Run(index, opts)
					if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
						t.Fatalf("run %d: %+v; %s", runNumber, s, firstRunProblem(run))
					}
					if runNumber == 1 && SnapshotPerfCounters().Phases.MemoryCacheHits == 0 {
						t.Fatal("second run did not hit the runtime memory cache")
					}
				}
			})
		}
	}
}

func TestQ3SharingReasonVariableMemberRejected(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			index := loadTestIndex(t, root)
			body, err := os.ReadFile("testdata/conformance/q3share/variable.apex")
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
			if err != nil {
				return
			}
			org := orgFromIndex(index)
			org.APIVersion = api
			runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			if _, err = runner.newMachine().Execute(program); err == nil {
				t.Fatal("accepted a member on a field-token variable")
			}
		})
	}
}
