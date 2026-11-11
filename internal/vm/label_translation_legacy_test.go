package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// An older persisted value has no exact-source marker; its original blank
// value may have been a nonbreaking character before the old loader trimmed it.
func TestUnmarkedLabelTranslationMetadataPreservesPriorBehavior(t *testing.T) {
	for _, translationFile := range []string{"", "fr.translation-meta.xml"} {
		t.Run(translationFile, func(t *testing.T) {
			original := storage.MetadataRegistry{Labels: []storage.LabelMetadata{
				{Name: "Greeting", Language: "en_US", Value: "Master", File: "CustomLabels.labels-meta.xml"},
				{Name: "Greeting", Language: "fr", Value: "", File: translationFile},
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
			value, ok := machine.resolveLabelMethodValue("", "Greeting", "fr")
			if !ok || value != "" {
				t.Fatalf("legacy value=%q found=%v", value, ok)
			}
			if !machine.labelTranslationExists("", "Greeting", "fr") {
				t.Fatal("legacy row unexpectedly changed")
			}
		})
	}
}
