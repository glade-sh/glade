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

func TestQ3CustomWrapperTimeNativeConformance(t *testing.T) {
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
		if row[2] != "custom_wrapper_time" {
			continue
		}
		api, route := row[0], row[1]
		key := api + "/" + route
		if seen[key] {
			t.Fatalf("duplicate %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			version := strings.TrimSuffix(api, ".0")
			testClass := "Q3WrapperTest" + version
			helper := "Q3WrapperProbe" + version
			for _, name := range []string{testClass, helper} {
				for _, ext := range []string{".cls", ".cls-meta.xml"} {
					b, err := os.ReadFile(filepath.Join(fixture, "wrapperclasses", name+ext))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/classes", name+ext), string(b))
				}
			}
			for _, objectFile := range []string{"Q3Item__c.object-meta.xml", "fields/When__c.field-meta.xml"} {
				b, err := os.ReadFile(filepath.Join(fixture, "wrapperobjects", "Q3Item__c", objectFile))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/objects/Q3Item__c", objectFile), string(b))
			}
			index := loadTestIndex(t, root)
			if route == "istest" {
				run := Run(index, Options{SelectedClasses: []string{testClass}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
				if s := run.Summary(); s.Total != 1 || s.Passed != 1 {
					t.Fatalf("named result: %+v; %s", s, firstRunProblem(run))
				}
				return
			}
			if route != "anonymous" {
				t.Fatalf("unknown route %s", route)
			}
			b, err := os.ReadFile(filepath.Join(fixture, "wrapperclasses", "anonymous"+version+".apex"))
			if err != nil {
				t.Fatal(err)
			}
			program, err := vm.CompileAnonymousWithOptions(string(b), vm.CompileOptions{APIVersion: api})
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
		t.Fatalf("missing native rows: %v", seen)
	}
}
