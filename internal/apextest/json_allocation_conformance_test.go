package apextest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestRuntimeJSONDeserializationAllocationConformance(t *testing.T) {
	var native struct {
		Versions []struct {
			API   string           `json:"api"`
			Cells []map[string]any `json:"cells"`
		} `json:"versions"`
	}
	raw, err := os.ReadFile("testdata/conformance/json_allocation/native.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	prefix, err := os.ReadFile("testdata/conformance/json_allocation/OwnedJsonAllocationProbe.cls.template")
	if err != nil {
		t.Fatal(err)
	}
	if len(native.Versions) != 3 {
		t.Fatalf("native versions = %d, want 3", len(native.Versions))
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
	for _, version := range native.Versions {
		t.Run(version.API, func(t *testing.T) {
			if len(version.Cells) != 13 {
				t.Fatalf("native cells = %d, want 13", len(version.Cells))
			}
			var body strings.Builder
			body.Write(prefix)
			body.WriteString("\n@IsTest static void replayNative(){\n")
			seen := map[string]bool{}
			for _, cell := range version.Cells {
				shape, ok := cell["shape"].(string)
				if !ok {
					t.Fatal("native shape missing")
				}
				strict, _ := cell["strict"].(bool)
				key := fmt.Sprintf("%s/%t", shape, strict)
				if seen[key] || cell["error"] != nil {
					t.Fatalf("duplicate or failed native cell %s", key)
				}
				seen[key] = true
				fmt.Fprintf(&body, "{Map<String,Object> cell=observe(%t,%s);\n", strict, literal(shape))
				fmt.Fprintf(&body, "System.assertEquals(false,cell.containsKey('error'),%s);\n", literal(key+" error"))
				keys := make([]string, 0, len(cell))
				for name := range cell {
					keys = append(keys, name)
				}
				sort.Strings(keys)
				for _, name := range keys {
					fmt.Fprintf(&body, "System.assertEquals(%s,cell.get(%s),%s);\n", literal(cell[name]), literal(name), literal(key+" "+name))
				}
				body.WriteString("}\n")
			}
			body.WriteString("}\n}\n")
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, version.API))
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			writeFile(t, filepath.Join(classes, "OwnedJsonAllocationProbe.cls"), body.String())
			writeFile(t, filepath.Join(classes, "OwnedJsonAllocationProbe.cls-meta.xml"), "<ApexClass><apiVersion>"+version.API+"</apiVersion></ApexClass>")
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"OwnedJsonAllocationProbe"}, NoDiskCache: true, Parallelism: 1})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				t.Fatalf("summary=%#v problem=%q", summary, firstRunProblem(run))
			}
		})
	}
}
