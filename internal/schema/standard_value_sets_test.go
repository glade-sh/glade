package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestProjectStandardValueSets(t *testing.T) {
	for _, row := range []struct{ valueSet, object, field string }{
		{"ContractStatus", "Contract", "Status"},
		{"OrderStatus", "Order", "Status"},
		{"TaskStatus", "Task", "Status"},
	} {
		for _, namespace := range []string{"", "probe"} {
			for _, source := range []string{"value_set_only", "snapshot_implicit_binding", "snapshot_explicit_binding"} {
				t.Run(row.valueSet+"/"+namespace+"/"+source, func(t *testing.T) {
					root := t.TempDir()
					valueSetPath := filepath.Join(root, "force-app/main/default/standardValueSets", row.valueSet+".standardValueSet-meta.xml")
					writeFile(t, valueSetPath, `<StandardValueSet xmlns="http://soap.sforce.com/2006/04/metadata"><standardValue><fullName>CustomStatus</fullName><default>false</default><label>Custom Status</label><isActive>true</isActive></standardValue></StandardValueSet>`)
					p := project.Project{
						Root:                  root,
						Namespace:             namespace,
						StandardValueSetFiles: []string{valueSetPath},
					}
					if source != "value_set_only" {
						field := Field{
							Name:               row.field,
							Type:               "Picklist",
							RestrictedPicklist: true,
							PicklistValues:     []PicklistValue{{FullName: "Baseline", Label: "Baseline", Active: true}},
						}
						if source == "snapshot_explicit_binding" {
							field.ValueSetName = row.valueSet
						}
						data, err := json.Marshal(Schema{Objects: []Object{{Name: row.object, Fields: []Field{field}}}})
						if err != nil {
							t.Fatal(err)
						}
						p.SchemaSnapshot = filepath.Join(root, "org.schema.json")
						writeFile(t, p.SchemaSnapshot, string(data))
						digest := sha256.Sum256(data)
						p.SchemaSnapshotSHA256 = hex.EncodeToString(digest[:])
					}

					loaded, err := LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					if len(loaded.Objects) != 1 || loaded.Objects[0].Name != row.object {
						t.Fatalf("standard object identity = %#v; want %s without a namespace", loaded.Objects, row.object)
					}
					fields := fieldsByName(loaded.Objects[0].Fields)
					field, ok := fields[row.field]
					if !ok {
						t.Fatalf("standard value set did not bind %s.%s: %#v", row.object, row.field, loaded.Objects[0].Fields)
					}
					if field.Name != row.field {
						t.Fatalf("standard field identity = %q; want %q", field.Name, row.field)
					}
					want := []PicklistValue{{FullName: "CustomStatus", Label: "Custom Status", Active: true}}
					if !reflect.DeepEqual(field.PicklistValues, want) {
						t.Fatalf("effective values = %#v; want project declarations %#v", field.PicklistValues, want)
					}
					if !field.StandardValueSetApplied {
						t.Fatal("lost deployed standard value set provenance")
					}
				})
			}
		}
	}
}

func TestEmptyProjectValueSetRetainsExistingValues(t *testing.T) {
	for _, name := range []string{"OrderStatus", "ContractStatus", "SyntheticSet"} {
		t.Run(name, func(t *testing.T) {
			want := []PicklistValue{{FullName: "Existing", Active: true}}
			field := Field{ValueSetName: name, PicklistValues: append([]PicklistValue(nil), want...)}
			applyValueSet(&field, map[string][]PicklistValue{strings.ToLower(name): {}})
			if !reflect.DeepEqual(field.PicklistValues, want) {
				t.Fatalf("empty value set changed existing values: %#v", field.PicklistValues)
			}
		})
	}
}
