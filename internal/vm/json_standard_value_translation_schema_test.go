package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A project override that makes Industry a plain string must not acquire the
// standard picklist translation merely because it shares the field name.
func TestJSONStandardValueTranslationRespectsActiveField(t *testing.T) {
	org := storage.OrgState{
		Features: []string{"TranslationLanguage-fr"},
		Metadata: storage.MetadataRegistry{ValueSetTranslations: []storage.StandardValueSetTranslation{{ValueSet: "Industry", Language: "fr", MasterLabel: "Technology", Translation: "Technologie"}}},
		Objects:  map[string]storage.ObjectState{"Account": {Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Industry": {APIName: "Industry", Type: storage.FieldString}}}}},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LanguageLocaleKey": storage.StringValue("fr")}})
	value := Object("Account")
	value.Fields["label"] = String("Technology")
	machine.applyJSONQueryFunctions(&value, []string{"toLabel(Industry) label"})
	if got := value.Fields["label"].Text; got != "Technology" {
		t.Fatalf("non-picklist field translated: %q", got)
	}
}
