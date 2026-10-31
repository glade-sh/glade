package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native translations at APIs 62 and 67: boundary-fr-org.tsv:7-13 and
// boundary-de-org.tsv:7-13. Status bindings use the captured mappings in
// standard_restricted_picklists/project-valuesets and the documented toLabel rule.
func TestJSONStandardValueTranslationLanguagesAndSets(t *testing.T) {
	for _, tc := range []struct{ object, field, set, language, master, translation string }{
		{"Account", "Industry", "Industry", "fr", "Agriculture", "Agriculture FR"},
		{"Account", "Industry", "Industry", "de", "Banking", "Bankwesen DE"},
		{"Opportunity", "StageName", "OpportunityStage", "fr", "Qualification", "Qualification FR"},
		{"Opportunity", "StageName", "OpportunityStage", "de", "Prospecting", "Akquise DE"},
		{"Task", "Status", "TaskStatus", "fr", "Not Started", "Pas commence"},
		{"Order", "Status", "OrderStatus", "fr", "Draft", "Brouillon"},
		{"Contract", "Status", "ContractStatus", "de", "Draft", "Entwurf"},
	} {
		org := storage.OrgState{
			Features: []string{"TranslationLanguage-" + tc.language},
			Metadata: storage.MetadataRegistry{ValueSetTranslations: []storage.StandardValueSetTranslation{{ValueSet: tc.set, Language: tc.language, MasterLabel: tc.master, Translation: tc.translation}}},
		}
		machine := New(nil)
		machine.SetOrg(&org)
		machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LanguageLocaleKey": storage.StringValue(tc.language)}})
		value := Object(tc.object)
		value.Fields["label"] = String(tc.master)
		machine.applyJSONQueryFunctions(&value, []string{"toLabel(" + tc.field + ") label"})
		if got := value.Fields["label"].Text; got != tc.translation {
			t.Errorf("%s/%s/%s = %q, want %q", tc.object, tc.language, tc.master, got, tc.translation)
		}
	}
}
