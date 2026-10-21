package apextest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

func TestJSONAliasAllocationNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/json_alias_allocation"
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
	if len(rows) != 49 || strings.Join(rows[0], ",") != "id,api,route,status,observed" {
		t.Fatalf("expected 48 native cells with their header, got %d rows", len(rows))
	}
	source, err := os.ReadFile(filepath.Join(fixture, "JsonAliasProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	literal := func(value any) string {
		switch value := value.(type) {
		case nil:
			return "null"
		case string:
			return conformanceApexString(value)
		case bool, float64:
			return fmt.Sprint(value)
		default:
			t.Fatalf("unexpected native value %T", value)
			return ""
		}
	}
	seen := map[string]bool{}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				var body strings.Builder
				cells := 0
				for _, row := range rows[1:] {
					if len(row) != 5 {
						t.Fatalf("invalid native row: %v", row)
					}
					if row[1] != api || row[2] != route {
						continue
					}
					key := strings.Join(row[:3], "/")
					if seen[key] || row[3] != "OBSERVED" {
						t.Fatalf("duplicate or unobserved native cell %s", key)
					}
					seen[key] = true
					cells++
					var expected map[string]any
					if err := json.Unmarshal([]byte(row[4]), &expected); err != nil {
						t.Fatal(err)
					}
					fmt.Fprintf(&body, "{Map<String,Object> observed=JsonAliasProbe.observe(%s,%s);\n", literal(expected["shape"]), literal(expected["strict"]))
					fmt.Fprintf(&body, "System.assertEquals(%d,observed.size(),%s);\n", len(expected), literal(row[0]+" fields"))
					keys := make([]string, 0, len(expected))
					for name := range expected {
						keys = append(keys, name)
					}
					sort.Strings(keys)
					for _, name := range keys {
						fmt.Fprintf(&body, "System.assertEquals(%s,observed.get(%s),%s);\n", literal(expected[name]), literal(name), literal(row[0]+" "+name))
					}
					body.WriteString("}\n")
				}
				if cells != 16 {
					t.Fatalf("native cells=%d, want 16", cells)
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				classes := filepath.Join(root, "force-app/main/default/classes")
				writeFile(t, filepath.Join(classes, "JsonAliasProbe.cls"), string(source))
				writeFile(t, filepath.Join(classes, "JsonAliasProbe.cls-meta.xml"), "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")

				index := loadTestIndex(t, root)
				if result := sema.AnalyzeAnonymous(index, body.String(), api); result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(body.String(), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	if len(seen) != 48 {
		t.Fatalf("native coverage=%d, want 48 cells", len(seen))
	}
}
