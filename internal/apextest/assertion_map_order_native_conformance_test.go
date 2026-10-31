package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestAssertionMapOrderNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "assertion_map_order")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(dir, "probe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	class, err := os.ReadFile(filepath.Join(dir, "project", "force-app", "main", "default", "classes", "MapOrderProbeTest.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			expected := map[string]string{}
			named := false
			for _, row := range rows[1:] {
				if len(row) != 4 {
					t.Fatal("invalid native row")
				}
				if row[0] == api {
					if row[1] == "anonymous" {
						expected[row[2]] = row[3]
					}
					if row[1] == "isTest" && row[2] == "assertions" && row[3] == "passed" {
						named = true
					}
				}
			}
			if len(expected) != 5 || !named {
				t.Fatal("incomplete native observations")
			}
			t.Run("anonymous", func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				result, err := vm.Execute(program, nil)
				if err != nil {
					t.Fatal(err)
				}
				observed := map[string]string{}
				for _, line := range result.Debug {
					if at := strings.Index(line, "CASE_"); at >= 0 {
						key, val, found := strings.Cut(line[at+5:], "=")
						if found {
							observed[key] = val
						}
					}
				}
				for key, want := range expected {
					if observed[key] != want {
						t.Errorf("%s: native %q, local %q", key, want, observed[key])
					}
				}
			})
			t.Run("isTest_project", func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"","sourceApiVersion":"`+api+`"}`)
				path := filepath.Join(root, "force-app", "main", "default", "classes", "MapOrderProbeTest.cls")
				writeFile(t, path, string(class))
				writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"MapOrderProbeTest"}, NoDiskCache: true, Parallelism: 1})
				summary := run.Summary()
				if summary.Total != 1 || summary.Passed != 1 {
					t.Fatalf("named native assertions did not pass: %+v; suites=%+v", summary, run.Suites)
				}
			})
		})
	}
}
