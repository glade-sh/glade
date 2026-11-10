package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

// Captured empty, missing and nonempty translation results remain true when
// metadata is restored and another language is present in the registry.
func TestEmptyLabelTranslationRestoredMetadata(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"packageDirectories":[{"path":"force-app","default":true}]}`,
		"force-app/main/default/labels/CustomLabels.labels-meta.xml": `<CustomLabels>
  <labels><fullName>Greeting</fullName><language>en_US</language><value>Master</value></labels>
  <labels><fullName>EarlyBlank</fullName><language>en_US</language><value>Early master</value></labels>
</CustomLabels>`,
		"force-app/main/default/translations/fr.translation-meta.xml": `<Translations>
  <customLabels><name>Greeting</name><label></label></customLabels>
  <customLabels><name>EarlyBlank</name><label>Autre</label></customLabels>
</Translations>`,
		"force-app/main/default/translations/de.translation-meta.xml": `<Translations>
  <customLabels><name>Greeting</name><label>Andere</label></customLabels>
  <customLabels><name>EarlyBlank</name><label></label></customLabels>
</Translations>`,
	}
	for path, content := range files {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	original, err := resource.LoadProjectWithDependencies(p)
	if err != nil {
		t.Fatal(err)
	}
	// LoadProjectWithDependencies sorts by name and language, so the German
	// entry precedes the en_US master and the French entry follows it.
	var languages []string
	for _, label := range original.Labels {
		if label.Name == "Greeting" {
			languages = append(languages, label.Language)
		}
	}
	if len(languages) != 3 || languages[0] != "de" || languages[1] != "en_US" || languages[2] != "fr" {
		t.Fatalf("unexpected sorted languages: %v", languages)
	}
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
	machine.executionUser.Fields["LanguageLocaleKey"] = String("de")
	if value, ok := machine.resolveLabelMethodValue("", "EarlyBlank", "de"); !ok || value != "Early master" {
		t.Fatalf("early blank value=%q found=%v", value, ok)
	}
	if machine.labelTranslationExists("", "EarlyBlank", "de") {
		t.Fatal("empty translation sorted before master was treated as present")
	}
}
