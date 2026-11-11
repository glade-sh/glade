package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
)

func TestCapturedNamespacedStaticFieldTypeCollision(t *testing.T) {
	const fixture = "testdata/native_conformance/static_field_type_collision_namespaced"
	project, err := os.ReadFile(filepath.Join(fixture, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal(project, &config); err != nil || config.Namespace == "" {
		t.Fatalf("invalid namespace config: %v", err)
	}
	rows := readScopeRunnerTSV(t, filepath.Join(fixture, "org.tsv"), []string{"case", "api", "route", "outcome"})
	if len(rows) != 4 {
		t.Fatalf("expected four native rows, got %d", len(rows))
	}
	for _, row := range rows {
		if len(row) != 4 || (row[1] != "62.0" && row[1] != "67.0") {
			t.Fatalf("invalid native row: %v", row)
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fixture, "cases", row[0]+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var classes map[string]string
			if err := json.Unmarshal(data, &classes); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+config.Namespace+`","sourceApiVersion":"`+row[1]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			testClass := ""
			for name, source := range classes {
				path := filepath.Join(root, "force-app/main/default/classes", name)
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><status>Active</status></ApexClass>`)
				if strings.Contains(source, "@IsTest") {
					testClass = strings.TrimSuffix(name, ".cls")
				}
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			rejected := index.HasErrors() || analysis.HasErrors()
			if row[0] == "missing_method" {
				if row[2] != "metadata-validation" || !strings.Contains(row[3], "Method does not exist") || !rejected {
					t.Fatalf("native rejection %q, local diagnostics %v %v", row[3], index.Diagnostics, analysis.Diagnostics)
				}
				return
			}
			if row[0] != "field_dispatch" || row[2] != "metadata-validation+test" || row[3] != "passed" || rejected || testClass == "" {
				t.Fatalf("native acceptance %v, local diagnostics %v %v", row, index.Diagnostics, analysis.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: "verify", NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
				t.Fatalf("native test passed; local runner: %+v", run.Summary())
			}
		})
	}
}

func TestCapturedStaticFieldTypeBoundary(t *testing.T) {
	const fixture = "testdata/native_conformance/static_field_type_boundary"
	project, err := os.ReadFile(filepath.Join(fixture, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal(project, &config); err != nil || config.Namespace == "" {
		t.Fatalf("invalid namespace config: %v", err)
	}
	rows := readScopeRunnerTSV(t, filepath.Join(fixture, "org.tsv"), []string{"case", "api", "route", "outcome"})
	if len(rows) != 6 {
		t.Fatalf("expected six native rows, got %d", len(rows))
	}
	for _, row := range rows {
		if len(row) != 4 || (row[1] != "62.0" && row[1] != "67.0") || row[2] != "metadata-validation" {
			t.Fatalf("invalid native row: %v", row)
		}
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(fixture, "cases", row[0]+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var classes map[string]string
			if err := json.Unmarshal(data, &classes); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+config.Namespace+`","sourceApiVersion":"`+row[1]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			for name, source := range classes {
				path := filepath.Join(root, "force-app/main/default/classes", name)
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+row[1]+`</apiVersion><status>Active</status></ApexClass>`)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			rejected := index.HasErrors() || analysis.HasErrors()
			if row[0] == "field_missing" {
				if !strings.Contains(row[3], "Static method cannot") || !rejected {
					t.Fatalf("native rejection %q, local diagnostics %v %v", row[3], index.Diagnostics, analysis.Diagnostics)
				}
				matched := false
				for _, d := range analysis.Diagnostics {
					if d.NativeMessage == row[3] {
						matched = true
					}
				}
				if !matched {
					t.Fatalf("native diagnostic %q missing from %v", row[3], analysis.Diagnostics)
				}
			} else if row[3] != "accepted" || rejected {
				t.Fatalf("native acceptance %v, local diagnostics %v %v", row, index.Diagnostics, analysis.Diagnostics)
			}
		})
	}
}

func TestCapturedStaticFieldTypeOverloads(t *testing.T) {
	const fixture = "testdata/native_conformance/static_field_type_overload"
	project, err := os.ReadFile(filepath.Join(fixture, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal(project, &config); err != nil || config.Namespace == "" {
		t.Fatalf("invalid namespace config: %v", err)
	}
	rows := readScopeRunnerTSV(t, filepath.Join(fixture, "org.tsv"), []string{"case", "api", "route", "outcome"})
	outcomes := map[string]string{}
	for _, row := range rows {
		if len(row) != 4 {
			t.Fatalf("invalid row %v", row)
		}
		key := strings.Join(row[:3], "/")
		if _, exists := outcomes[key]; exists {
			t.Fatalf("duplicate row %s", key)
		}
		outcomes[key] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"overload_mismatch", "overload_match", "type_control"} {
			t.Run(name+"/"+api, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(fixture, "cases", name+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var classes map[string]string
				if err := json.Unmarshal(data, &classes); err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+config.Namespace+`","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				testClass := ""
				for filename, source := range classes {
					path := filepath.Join(root, "force-app/main/default/classes", filename)
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
					if strings.Contains(source, "@IsTest") {
						testClass = strings.TrimSuffix(filename, ".cls")
					}
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				rejected := index.HasErrors() || analysis.HasErrors()
				if name == "overload_mismatch" {
					want := outcomes[name+"/"+api+"/metadata-diagnostic"]
					matched := false
					for _, d := range analysis.Diagnostics {
						if d.NativeMessage == want {
							matched = true
						}
					}
					if want == "" || !rejected || !matched {
						t.Fatalf("native diagnostic %q; local diagnostics %v %v", want, index.Diagnostics, analysis.Diagnostics)
					}
					return
				}
				if outcomes[name+"/"+api+"/metadata-compilation"] != "accepted" || outcomes[name+"/"+api+"/named-test"] != "passed" || rejected || testClass == "" {
					t.Fatalf("native success; local diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				run := Run(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: "verify", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("native test passed; local runner: %+v", run.Summary())
				}
			})
		}
	}
}

func TestCapturedStaticFieldTypeInherited(t *testing.T) {
	const fixture = "testdata/native_conformance/static_field_type_inherited"
	project, err := os.ReadFile(filepath.Join(fixture, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal(project, &config); err != nil || config.Namespace == "" {
		t.Fatalf("invalid namespace config: %v", err)
	}
	rows := readScopeRunnerTSV(t, filepath.Join(fixture, "org.tsv"), []string{"case", "api", "route", "outcome"})
	outcomes := map[string]string{}
	for _, row := range rows {
		if len(row) != 4 {
			t.Fatalf("invalid row %v", row)
		}
		key := strings.Join(row[:3], "/")
		if _, exists := outcomes[key]; exists {
			t.Fatalf("duplicate row %s", key)
		}
		outcomes[key] = row[3]
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"inherited_missing", "inherited_valid"} {
			t.Run(name+"/"+api, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(fixture, "cases", name+".json"))
				if err != nil {
					t.Fatal(err)
				}
				var classes map[string]string
				if err := json.Unmarshal(data, &classes); err != nil {
					t.Fatal(err)
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"`+config.Namespace+`","sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				testClass := ""
				for filename, source := range classes {
					path := filepath.Join(root, "force-app/main/default/classes", filename)
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
					if strings.Contains(source, "@IsTest") {
						testClass = strings.TrimSuffix(filename, ".cls")
					}
				}
				index := loadTestIndex(t, root)
				analysis := sema.Analyze(index)
				rejected := index.HasErrors() || analysis.HasErrors()
				if name == "inherited_missing" {
					want := outcomes[name+"/"+api+"/metadata-diagnostic"]
					matched := false
					for _, d := range analysis.Diagnostics {
						if d.NativeMessage == want {
							matched = true
						}
					}
					if want == "" || !rejected || !matched {
						t.Fatalf("native diagnostic %q; local diagnostics %v %v", want, index.Diagnostics, analysis.Diagnostics)
					}
					return
				}
				if outcomes[name+"/"+api+"/metadata-compilation"] != "accepted" || outcomes[name+"/"+api+"/named-test"] != "passed" || rejected || testClass == "" {
					t.Fatalf("native success; local diagnostics %v %v", index.Diagnostics, analysis.Diagnostics)
				}
				run := Run(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: "verify", NoDiskCache: true, Parallelism: 1})
				if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem != nil {
					t.Fatalf("native test passed; local runner: %+v", run.Summary())
				}
			})
		}
	}
}
