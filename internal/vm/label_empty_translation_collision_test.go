package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A managed label sharing a name cannot satisfy the local blank translation.
func TestEmptyLocalLabelTranslationIgnoresManagedCollision(t *testing.T) {
	org := storage.NewOrgState()
	org.Metadata.Labels = []storage.LabelMetadata{
		{Name: "Greeting", Namespace: "pkg", Language: "fr", Value: "Managed", File: "fr.translation-meta.xml"},
		{Name: "Greeting", Language: "en_US", Value: "Local master", File: "CustomLabels.labels-meta.xml"},
		{Name: "Greeting", Language: "fr", Value: "", File: "fr.translation-meta.xml"},
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.executionUser = Object("User")
	machine.executionUser.Fields["LanguageLocaleKey"] = String("fr")
	value, ok := machine.resolveLabelMethodValue("", "Greeting", "fr")
	if !ok || value != "Local master" {
		t.Fatalf("value=%q found=%v", value, ok)
	}
	if machine.labelTranslationExists("", "Greeting", "fr") {
		t.Fatal("managed translation satisfied a local label")
	}
}
