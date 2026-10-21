package apextest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestRuntimeInstanceInitializerConformance(t *testing.T) {
	var native []struct {
		API   string           `json:"sourceApi"`
		Cells []map[string]any `json:"cells"`
	}
	raw, err := os.ReadFile("testdata/conformance/instance_initializer/native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	prefix, err := os.ReadFile("testdata/conformance/instance_initializer/OwnedInstanceInitializerProbe.cls.template")
	if err != nil {
		t.Fatal(err)
	}
	if len(native) != 3 {
		t.Fatalf("native versions = %d, want 3", len(native))
	}
	var variants []struct {
		API   string           `json:"sourceApi"`
		Cells []map[string]any `json:"cells"`
	}
	raw, err = os.ReadFile("testdata/conformance/instance_initializer/native_unit2.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &variants); err != nil {
		t.Fatal(err)
	}
	variantByAPI := map[string]map[string]any{}
	for _, version := range variants {
		if len(version.Cells) != 1 || version.Cells[0]["shape"] != "upper-final" {
			t.Fatal("expected one native variant cell")
		}
		variantByAPI[version.API] = version.Cells[0]
	}
	literal := func(value any) string {
		switch value := value.(type) {
		case nil:
			return "null"
		case bool:
			return strconv.FormatBool(value)
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		case string:
			return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(value) + "'"
		default:
			t.Fatalf("unexpected native value type %T", value)
			return ""
		}
	}
	for _, version := range native {
		t.Run(version.API, func(t *testing.T) {
			if len(version.Cells) != 5 {
				t.Fatalf("native cells = %d, want 5", len(version.Cells))
			}
			var body strings.Builder
			body.Write(prefix)
			body.WriteString("\n@IsTest static void replayNative(){\n")
			seen := map[string]bool{}
			for _, cell := range version.Cells {
				shape, ok := cell["shape"].(string)
				if !ok || seen[shape] {
					t.Fatal("missing or duplicate native shape")
				}
				seen[shape] = true
				fmt.Fprintf(&body, "{Map<String,Object> cell=observe(%s);\n", literal(shape))
				fmt.Fprintf(&body, "System.assertEquals(%d,cell.size(),%s);\n", len(cell), literal(shape+" keys"))
				keys := make([]string, 0, len(cell))
				for name := range cell {
					keys = append(keys, name)
				}
				sort.Strings(keys)
				for _, name := range keys {
					fmt.Fprintf(&body, "System.assertEquals(%s,cell.get(%s),%s);\n", literal(cell[name]), literal(name), literal(shape+" "+name))
				}
				body.WriteString("}\n")
			}
			body.WriteString("}\n}\n")
			for _, units := range []int{1, 2} {
				t.Run(fmt.Sprintf("source-units-%d", units), func(t *testing.T) {
					root := t.TempDir()
					for unit := 0; unit < units; unit++ {
						source := body.String()
						if unit == 1 {
							variant := variantByAPI[version.API]
							if variant == nil {
								t.Fatal("native variant missing")
							}
							source = strings.Replace(source, "public final Integer OFFSET=3;\n  public final Integer SCALE=7;\n  public final Integer VALUE=", fmt.Sprintf("public final Integer OFFSET=%s;\n  public final Integer SCALE=%s;\n  public final Integer VALUE=", literal(variant["first"]), literal(variant["second"])), 1)
							for _, cell := range version.Cells {
								if cell["shape"] != "upper-final" {
									continue
								}
								for name, value := range variant {
									before := fmt.Sprintf("System.assertEquals(%s,cell.get(%s),%s);", literal(cell[name]), literal(name), literal("upper-final "+name))
									after := fmt.Sprintf("System.assertEquals(%s,cell.get(%s),%s);", literal(value), literal(name), literal("upper-final "+name))
									source = strings.Replace(source, before, after, 1)
								}
							}
						}
						base := filepath.Join(root, fmt.Sprint(unit))
						writeFile(t, filepath.Join(base, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, version.API))
						classes := filepath.Join(base, "force-app", "main", "default", "classes")
						writeFile(t, filepath.Join(classes, "OwnedInstanceInitializerProbe.cls"), source)
						writeFile(t, filepath.Join(classes, "OwnedInstanceInitializerProbe.cls-meta.xml"), "<ApexClass><apiVersion>"+version.API+"</apiVersion></ApexClass>")
					}
					index := loadTestIndex(t, root)
					options := Options{NoDiskCache: true, Parallelism: 1}
					cases := Discover(index, options)
					if len(cases) != units {
						t.Fatalf("discovered %d contexts, want %d", len(cases), units)
					}
					run := RunCasesContext(context.Background(), index, options, cases)
					if got := run.Summary(); got.Total != units || got.Passed != units {
						t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
					}
				})
			}
		})
	}
}
