package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestLoadProjectMixedCustomTabsNativeFields(t *testing.T) {
	const native = "../apextest/testdata/conformance/recent_access_fields"
	shapes := []string{"Page", "Web", "Lwc", "Empty", "Truncated", "WrongRoot"}
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range append([]string{"all"}, shapes...) {
			t.Run(api+"/schema_load/"+shape, func(t *testing.T) {
				root := t.TempDir()
				if err := os.CopyFS(root, os.DirFS(native)); err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
				selected := shapes
				if shape != "all" {
					selected = []string{shape}
				}
				for _, name := range selected {
					relative := filepath.Join("force-app/main/default/tabs", name+".tab-meta.xml")
					data, err := os.ReadFile(filepath.Join("testdata/mixed_custom_tabs", relative))
					if err != nil {
						t.Fatal(err)
					}
					writeFile(t, filepath.Join(root, relative), string(data))
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				if len(p.TabFiles) != 2+len(selected) {
					t.Fatalf("discovered %d tabs, want %d", len(p.TabFiles), 2+len(selected))
				}
				loaded, err := LoadProject(p)
				if err != nil {
					t.Fatalf("schema load with %s tab metadata: %v", shape, err)
				}
				fields := map[string]Field{}
				for _, object := range loaded.Objects {
					for _, field := range object.Fields {
						fields[object.Name+"."+field.Name] = field
					}
				}
				// Reuse the native field-presence, type and writeability rows. Query
				// observations are exercised by the Apex conformance route separately.
				for _, route := range []string{"anonymous", "istest"} {
					dir := filepath.Join(native, "api"+api[:2], "native", route)
					run, err := os.ReadFile(filepath.Join(dir, "RUN-FACTS.txt"))
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(string(run), "api="+api+"\n") || !strings.Contains(string(run), "native_complete=true\n") {
						t.Fatal("incomplete native capture")
					}
					data, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
					if err != nil {
						t.Fatal(err)
					}
					rows := strings.Split(strings.TrimSpace(string(data)), "\n")
					checked := 0
					for _, line := range rows[1:] {
						row := strings.SplitN(line, "\t", 5)
						if len(row) != 5 || row[2] != route || row[3] != "OBSERVED" {
							t.Fatalf("invalid native row: %q", row)
						}
						dot := strings.LastIndex(row[0], ".")
						field, present := fields[row[0][:dot]]
						var got any
						switch row[0][dot+1:] {
						case "present":
							got = present
						case "type":
							got = strings.ToUpper(field.Type)
						case "writable":
							got = field.Createable == nil || *field.Createable || field.Updateable == nil || *field.Updateable
						case "query":
							continue
						default:
							t.Fatalf("unknown native row: %s", row[0])
						}
						var want any
						if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
							t.Fatal(err)
						}
						if got != want {
							t.Errorf("%s/%s: got %v, native %v", route, row[0], got, want)
						}
						checked++
					}
					if checked != 14 {
						t.Fatalf("checked %d native field rows, want 14", checked)
					}
				}
			})
		}
	}
}
