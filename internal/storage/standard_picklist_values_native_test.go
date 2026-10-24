package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStandardPicklistValuesNativeCatalogAndOverlayRoutes(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		data, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "conformance", "standard_picklist_values", "native", "api"+api, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
			row := strings.SplitN(line, "\t", 4)
			if len(row) != 4 {
				t.Fatal("invalid native row")
			}
			if row[1] != "anonymous" {
				continue
			}
			object, field := "", ""
			switch row[2] {
			case "opportunity":
				object, field = "Opportunity", "Type"
			case "lead":
				object, field = "Lead", "LeadSource"
			case "account":
				object, field = "Account", "Industry"
			default:
				continue
			}
			var want struct {
				Entries    [][]any `json:"entries"`
				Restricted bool    `json:"restricted"`
			}
			if err := json.Unmarshal([]byte(row[3]), &want); err != nil {
				t.Fatal(err)
			}
			if len(want.Entries) == 0 {
				t.Fatal("empty native list")
			}
			for _, route := range []string{"catalog", "packedDescribe", "bareOverlay", "persistedOverlay"} {
				t.Run(api+"/"+object+"."+field+"/"+route, func(t *testing.T) {
					entry, ok := standardObjectCatalogEntryForName(object)
					if !ok {
						t.Fatal("missing standard catalog entry")
					}
					got := entry.Definition.Fields[field]
					if route == "packedDescribe" {
						packed, found, err := lookupStandardDescribeCatalogV2(object)
						if err != nil || !found {
							t.Fatalf("missing packed describe: %v", err)
						}
						fieldFound := false
						for _, candidate := range packed.Fields {
							if candidate.Name == field {
								got = describeField(candidate)
								fieldFound = true
							}
						}
						if !fieldFound {
							t.Fatal("missing packed field: " + field)
						}
					}
					if route != "catalog" && route != "packedDescribe" {
						definition := ObjectDefinition{APIName: object}
						EnsureStandardObjectFields(&definition)
						if route == "persistedOverlay" {
							existing := definition.Fields[field]
							existing.PicklistValues = nil
							existing.PicklistValuesConfigured = false
							definition.Fields[field] = existing
							if !RefreshStandardObjectFieldOverlay(&definition) {
								t.Fatal("stale persisted picklist was not repaired")
							}
						}
						got = definition.Fields[field]
					}
					entries := make([][]any, 0, len(got.PicklistValues))
					for _, value := range got.PicklistValues {
						entries = append(entries, []any{value.Value, value.Label, value.Active, value.Default})
					}
					if !reflect.DeepEqual(entries, want.Entries) || got.RestrictedPicklist != want.Restricted {
						t.Errorf("native=%v restricted=%v; Glade=%v restricted=%v", want.Entries, want.Restricted, entries, got.RestrictedPicklist)
					}
				})
			}
		}
	}
}
