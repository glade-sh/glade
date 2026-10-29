package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// commerce_extension_shapes/org.tsv:50,61,111,122 observes WebStore's
// explicit-null constructor read as false at API62/67. Contact's equivalent
// direct/dynamic observations are the explicitNullConstructor rows in
// ../apextest/testdata/conformance/checkbox_dynamic_get_native.json.
// Catalog-only and installed schemas must expose the same dotted read without
// materializing an org object or replacing its raw null. WebStore dynamic null
// and case-insensitive spelling are compatibility controls, not new native rows.
func TestCommerceBooleanNullNativeConstructorCatalogAndInstalled(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ object, field string }{
			{"WebStore", "OptionsAutoFacetingEnabled"},
			{"Contact", "DoNotCall"},
		} {
			for _, installed := range []bool{false, true} {
				for _, spelling := range []string{tc.field, strings.ToLower(tc.field)} {
					t.Run(fmt.Sprintf("%s/%s/installed=%t/%s", api, tc.object, installed, spelling), func(t *testing.T) {
						org := storage.NewOrgState()
						org.Features = []string{"B2BCommerce", "Communities", "CommerceEnabled"}
						delete(org.Objects, tc.object)
						if installed {
							definition := storage.ObjectDefinition{APIName: tc.object}
							storage.EnsureStandardObjectFieldsForFeatures(&definition, org.Features)
							org.Objects[tc.object] = storage.ObjectState{Definition: definition}
						}
						machine := New(nil)
						machine.SetOrg(&org)
						program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
%[1]s row = new %[1]s(%[2]s=null);
Boolean observed = row.%[3]s;
%[1]s enabled = new %[1]s(%[2]s=true);
System.assertEquals(true, enabled.%[3]s);
%[1]s disabled = new %[1]s(%[2]s=false);
System.assertEquals(false, disabled.%[3]s);
`, tc.object, spelling, tc.field), CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						result, err := machine.Execute(program)
						if err != nil {
							t.Fatal(err)
						}
						if got := result.Vars["observed"]; got.Kind != ValueBool || got.Bool {
							t.Errorf("constructor dotted null read=%v, want false", got)
						}
						row := result.Vars["row"]
						if raw, ok := row.Fields[tc.field]; !ok || raw.Kind != ValueNull {
							t.Errorf("raw canonical field=%v present=%t, want stored null", raw, ok)
						}
						got, handled, err := machine.callSObjectMember(row, "get", []Value{String(tc.field)})
						if err != nil || !handled || got.Kind != ValueNull {
							t.Errorf("dynamic raw-null read=%v handled=%t err=%v", got, handled, err)
						}
						if _, exists := machine.Org.Objects[tc.object]; exists != installed {
							t.Errorf("constructor changed installed-object membership: got %t want %t", exists, installed)
						}
					})
				}
			}
		}
	}
}
