package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaReferenceGateFeatureNativeReceiverRoutes(t *testing.T) {
	var data struct {
		Features []struct {
			API, Query, Apex string
			Enabled          bool
		}
	}
	raw, err := os.ReadFile("../sema/testdata/schema_relationship_gate/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Features) != 4 {
		t.Fatal("missing feature/native API pairs")
	}
	rowsFile, err := os.Open("../sema/testdata/schema_relationship_gate/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer rowsFile.Close()
	reader := csv.NewReader(rowsFile)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	oracle := map[string]bool{}
	for _, row := range rows[1:] {
		oracle[strings.Join(row[:3], "/")] = row[3] == "true"
	}
	for _, c := range data.Features {
		for route, source := range map[string]string{"query": c.Query, "apex": c.Apex} {
			key := c.API + "/person_contact_" + map[bool]string{false: "off", true: "on"}[c.Enabled] + "_" + route + "/"
			for nativeRoute, want := range map[string]bool{"missing_metadata_deploy": false, "loaded_metadata_deploy": c.Enabled} {
				if got, ok := oracle[key+nativeRoute]; !ok || got != want {
					t.Fatalf("missing or inconsistent native row %s%s", key, nativeRoute)
				}
			}
			if c.Enabled && !oracle[key+"istest"] {
				t.Fatalf("native named route did not pass: %s", key)
			}
			t.Run(c.API+"/"+route+map[bool]string{false: "/disabled", true: "/enabled"}[c.Enabled], func(t *testing.T) {
				root := t.TempDir()
				file := filepath.Join(root, "Probe.cls")
				writeFile(t, file, source)
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: c.API, ApexFiles: []string{file}}, schema.Schema{})
				if c.Enabled {
					index.OrgShapeFeatures = []string{"PersonAccounts"}
				}
				units := newCompileGateUnits(index)
				for _, repeat := range []string{"cold", "shared_a", "shared_b"} {
					t.Run(repeat, func(t *testing.T) {
						t.Parallel()
						for lookup := 0; lookup < 2; lookup++ {
							if got := compileGateReferencesSchemaObject(file, source, units, "Contact"); got != c.Enabled {
								t.Fatalf("lookup=%d PersonContact owner referenced=%t enabled=%t", lookup, got, c.Enabled)
							}
						}
					})
				}
			})
		}
	}
}
