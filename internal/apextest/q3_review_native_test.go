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

func TestQ3SharingShadowedRootsNative(t *testing.T) {
	const fixture = "testdata/conformance/q3share/shadow"
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				version := strings.TrimSuffix(api, ".0")
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				for _, name := range []string{"Q3Shadow" + version, "Q3ShadowTest" + version} {
					for _, ext := range []string{".cls", ".cls-meta.xml"} {
						b, err := os.ReadFile(filepath.Join(fixture, "classes", name+ext))
						if err != nil {
							t.Fatal(err)
						}
						writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+ext), string(b))
					}
				}
				index := loadTestIndex(t, root)
				if route == "istest" {
					run := Run(index, Options{SelectedClasses: []string{"Q3ShadowTest" + version}, SelectedMethod: "observed", NoDiskCache: true, Parallelism: 1})
					if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
						t.Fatalf("%+v %s", s, firstRunProblem(run))
					}
					return
				}
				body, err := os.ReadFile(filepath.Join(fixture, "shadow"+version+".apex"))
				if err != nil {
					t.Fatal(err)
				}
				p, err := vm.CompileAnonymousWithOptions(string(body), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.newMachine().Execute(p); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestQ3SharingCatalogNative(t *testing.T) {
	f, err := os.Open("testdata/conformance/q3share/catalog.tsv")
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
	if len(rows) != 105 {
		t.Fatalf("captured rows=%d", len(rows)-1)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				var source strings.Builder
				count := 0
				for _, row := range rows[1:] {
					if row[0] != api {
						continue
					}
					if row[3] != row[2] || row[4] != row[2] {
						t.Fatalf("bad capture %v", row)
					}
					fmt.Fprintf(&source, "System.assertEquals('%s',String.valueOf(Schema.%s.RowCause.%s));\n", row[2], row[1], row[2])
					count++
				}
				if count != 52 {
					t.Fatalf("missing cases: %d", count)
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				name := "Q3CatalogProbe"
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls"), "@IsTest private class "+name+" { @IsTest static void observed() {\n"+source.String()+"\n} }")
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+".cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				index := loadTestIndex(t, root)
				if route == "istest" {
					run := Run(index, Options{SelectedClasses: []string{name}, SelectedMethod: "observed", NoDiskCache: true, Parallelism: 1})
					if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
						t.Fatalf("%+v %s", s, firstRunProblem(run))
					}
					return
				}
				p, err := vm.CompileAnonymousWithOptions(source.String(), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.newMachine().Execute(p); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestQ3SharingRejectedNamedClasses(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, prefix := range []string{"Q3ShareReject", "Q3VariableReject"} {
			t.Run(api+"/"+prefix, func(t *testing.T) {
				version := strings.TrimSuffix(api, ".0")
				name := prefix + version
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				for _, ext := range []string{".cls", ".cls-meta.xml"} {
					b, err := os.ReadFile(filepath.Join("testdata/conformance/q3share/rejectclasses", name+ext))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+ext), string(b))
				}
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{name}, NoDiskCache: true, Parallelism: 1})
				s := run.Summary()
				if s.CompileErrors != 1 || !strings.Contains(firstRunProblem(run), "Variable does not exist") {
					t.Fatalf("native compile rejection not matched: %+v %s", s, firstRunProblem(run))
				}
			})
		}
	}
}
