package orgdescribe

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

func TestImportedPicklistProvenanceRoutes(t *testing.T) {
	for _, item := range []struct{ object, field string }{{"Asset", "StatusReason"}, {"Contact", "GenderIdentity"}, {"Contact", "Pronouns"}, {"Contract", "ShippingGeocodeAccuracy"}, {"Event", "Type"}, {"Lead", "GenderIdentity"}, {"Lead", "Pronouns"}, {"Task", "Type"}} {
		for _, tc := range []struct {
			name, list string
			configured bool
			length     int
		}{{"empty", `[]`, true, 0}, {"populated", `[{"value":"Owned","label":"Owned","active":true}]`, true, 1}, {"missing", "", false, -1}, {"null", "null", false, -1}} {
			t.Run(item.object+"/"+item.field+"/"+tc.name, func(t *testing.T) {
				jsonField := fmt.Sprintf(`{"name":%q,"type":"picklist"`, item.field)
				if tc.list != "" {
					jsonField += `,"picklistValues":` + tc.list
				}
				jsonField += `}`
				var field Field
				if err := json.Unmarshal([]byte(jsonField), &field); err != nil {
					t.Fatal(err)
				}
				obj := SObject{Name: item.object, Fields: []Field{field}}
				check := func(route string, def storage.ObjectDefinition) {
					t.Helper()
					storage.EnsureStandardObjectFields(&def)
					got := def.Fields[item.field]
					if got.PicklistValuesConfigured != tc.configured {
						t.Errorf("%s configured=%v want=%v", route, got.PicklistValuesConfigured, tc.configured)
					}
					if tc.length >= 0 && len(got.PicklistValues) != tc.length {
						t.Errorf("%s values=%v want length %d", route, got.PicklistValues, tc.length)
					}
					if tc.length < 0 && len(got.PicklistValues) == 0 {
						t.Errorf("%s did not repair missing list", route)
					}
				}
				check("direct", obj.ToObjectDefinition())
				encodedImport, err := json.Marshal(obj)
				if err != nil {
					t.Fatal(err)
				}
				var restoredImport SObject
				if err = json.Unmarshal(encodedImport, &restoredImport); err != nil {
					t.Fatal(err)
				}
				check("import snapshot", restoredImport.ToObjectDefinition())
				original := Catalog{Objects: []SObject{restoredImport}}.ToSchema()
				snapshot, err := json.Marshal(original)
				if err != nil {
					t.Fatal(err)
				}
				var restored schema.Schema
				if err = json.Unmarshal(snapshot, &restored); err != nil {
					t.Fatal(err)
				}
				registry := sobject.BuildDescribeRegistry(restored)
				desc, err := registry.Describe(item.object)
				if err != nil {
					t.Fatal(err)
				}
				check("schema snapshot", sobject.ToObjectDefinition(desc))
			})
		}
	}
}
