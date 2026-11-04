package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestDependencyPrivateReflectionNativeConformance(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "dependency_private_reflection")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 {
		t.Fatalf("expected four native rows, got %d", len(rows)-1)
	}
	source, err := os.ReadFile(filepath.Join(dir, "project", "force-app", "main", "default", "classes", "Case245ReflectionCapture.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			var expected map[string]string
			if err := json.Unmarshal([]byte(row[2]), &expected); err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			dep, consumer := filepath.Join(root, "dep"), filepath.Join(root, "consumer")
			writeFile(t, filepath.Join(dep, "sfdx-project.json"), fmt.Sprintf(`{"namespace":"pkg","sourceApiVersion":"%s","packageDirectories":[{"path":"force-app","default":true}]}`, row[0]))
			dependencySource := strings.Replace(string(source), "public virtual class Case245ReflectionCapture", "global virtual class Case245ReflectionCapture", 1)
			dependencySource = strings.Replace(dependencySource, "public static Map<String,String> observe()", "global static Map<String,String> observe()", 1)
			writeFile(t, filepath.Join(dep, "force-app/main/classes/Case245ReflectionCapture.cls"), dependencySource)
			writeFile(t, filepath.Join(consumer, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":"%s","packageDirectories":[{"path":"force-app","default":true}]}`, row[0]))
			writeFile(t, filepath.Join(consumer, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:../dep:1.0\"]\n")
			assertions := "Map<String,String> observed = pkg.Case245ReflectionCapture.observe();\n"
			for _, name := range []string{"privateSubclass", "privateOther", "publicSubclass", "publicBoundary"} {
				value, ok := expected[name]
				if !ok {
					t.Fatalf("missing native field %s", name)
				}
				assertions += fmt.Sprintf("System.assertEquals('%s', observed.get('%s'));\n", value, name)
			}
			if row[1] == "isTest" {
				writeFile(t, filepath.Join(consumer, "force-app/main/classes/ReflectionNativeTest.cls"), "@IsTest private class ReflectionNativeTest { @IsTest static void runCase() {\n"+assertions+"} }")
				run := Run(loadTestIndex(t, consumer), Options{Parallelism: 1, SelectedClasses: []string{"ReflectionNativeTest"}})
				if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
					t.Fatalf("summary=%+v suites=%+v", got, run.Suites)
				}
			} else if row[1] == "anonymous" {
				program, err := vm.CompileAnonymousWithOptions(assertions, vm.CompileOptions{APIVersion: row[0]})
				if err != nil {
					t.Fatal(err)
				}
				runner := newConformanceRunner(t, loadTestIndex(t, consumer), conformanceRunnerOptions{LinkProject: true})
				for i := 0; i < 2; i++ {
					if _, err := runner.execute(program); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				t.Fatalf("unknown route %q", row[1])
			}
		})
	}
}
