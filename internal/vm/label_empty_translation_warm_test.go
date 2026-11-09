package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Captured empty, missing and nonempty translation results remain true when
// metadata is restored and another language is present in the registry.
func TestEmptyLabelTranslationRestoredMetadata(t *testing.T) {
	original := storage.MetadataRegistry{Labels: []storage.LabelMetadata{
		{Name: "Greeting", Language: "en_US", Value: "Master"},
		{Name: "Greeting", Language: "fr", Value: ""},
		{Name: "Greeting", Language: "de", Value: "Andere"},
	}}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.MetadataRegistry
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Metadata = restored
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = Object("User")
	machine.executionUser.Fields["LanguageLocaleKey"] = String("fr")
	for _, tc := range []struct {
		language, want string
		translated     bool
	}{
		{"fr", "Master", false}, {"de", "Andere", true}, {"es", "Master", false},
	} {
		t.Run(tc.language, func(t *testing.T) {
			value, ok := machine.resolveLabelMethodValue("", "Greeting", tc.language)
			if !ok || value != tc.want {
				t.Fatalf("value=%q found=%v want %q", value, ok, tc.want)
			}
			if got := machine.labelTranslationExists("", "Greeting", tc.language); got != tc.translated {
				t.Fatalf("translationExists=%v want %v", got, tc.translated)
			}
		})
	}
}
