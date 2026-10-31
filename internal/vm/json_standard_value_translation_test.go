package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Active French toLabel alias and unaliased rows are in
// sobject_json3/org.tsv:30-33. An inactive French capture retained the master
// label (the previous fixture commit a recorded revision, org.tsv:22-25).
func TestJSONStandardValueTranslationActivation(t *testing.T) {
	for _, tc := range []struct {
		name, language, feature, expression, slot, want string
	}{
		{"active", "fr", "TranslationLanguage-fr", "toLabel(Industry)", "toLabel(Industry)", "Technologie"},
		{"activeAlias", "fr", "TranslationLanguage-fr", "toLabel(Industry) label", "label", "Technologie"},
		{"inactive", "fr", "", "toLabel(Industry) label", "label", "Technology"},
		{"otherLanguage", "de", "TranslationLanguage-fr", "toLabel(Industry) label", "label", "Technology"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			org := storage.OrgState{Metadata: storage.MetadataRegistry{ValueSetTranslations: []storage.StandardValueSetTranslation{{ValueSet: "Industry", Language: "fr", MasterLabel: "Technology", Translation: "Technologie"}}}}
			if tc.feature != "" {
				org.Features = []string{tc.feature}
			}
			machine.SetOrg(&org)
			machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"LanguageLocaleKey": storage.StringValue(tc.language)}})
			value := Object("Account")
			value.Fields[tc.slot] = String("Technology")
			machine.applyJSONQueryFunctions(&value, []string{tc.expression})
			if got := value.Fields[tc.slot].Text; got != tc.want {
				t.Fatalf("label = %q, want %q", got, tc.want)
			}
		})
	}
}
