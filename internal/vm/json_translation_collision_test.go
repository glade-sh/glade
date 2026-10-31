package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The query executor passes a projected master label to the VM, not the API
// value. The three cyclic collisions are captured in
// sobject_json3_collision/org.tsv:2-4 and API/route twins.
func TestJSONStandardValueTranslationUsesProjectedMasterLabel(t *testing.T) {
	options := []storage.PicklistValue{{Value: "Agriculture", Label: "Banking"}, {Value: "Banking", Label: "Technology"}, {Value: "Technology", Label: "Agriculture"}}
	translations := []storage.StandardValueSetTranslation{
		{ValueSet: "Industry", Language: "fr", MasterLabel: "Banking", Translation: "Premier FR"},
		{ValueSet: "Industry", Language: "fr", MasterLabel: "Technology", Translation: "Deuxieme FR"},
		{ValueSet: "Industry", Language: "fr", MasterLabel: "Agriculture", Translation: "Troisieme FR"},
	}
	org := storage.OrgState{Features: []string{"TranslationLanguage-fr"}, Metadata: storage.MetadataRegistry{ValueSetTranslations: translations}, Objects: map[string]storage.ObjectState{
		"Account": {Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Industry": {APIName: "Industry", Type: storage.FieldPicklist, PicklistValues: options}}}},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LanguageLocaleKey": storage.StringValue("fr")}})
	for _, tc := range []struct{ master, want string }{{"Banking", "Premier FR"}, {"Technology", "Deuxieme FR"}, {"Agriculture", "Troisieme FR"}} {
		for _, expression := range []string{"toLabel(Industry) label", "toLabel(Industry)"} {
			value := Object("Account")
			slot := "label"
			if expression == "toLabel(Industry)" {
				slot = "Industry"
			}
			value.Fields[slot] = String(tc.master)
			machine.applyJSONQueryFunctions(&value, []string{expression})
			if got := value.Fields[slot].Text; got != tc.want {
				t.Errorf("%s/%s got %q, want %q", expression, tc.master, got, tc.want)
			}
		}
	}
}
