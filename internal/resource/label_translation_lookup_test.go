package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A translation file can carry an untranslated placeholder for a label. The
// master value from CustomLabels must still win for a default-language lookup,
// whatever languages sort after the master language.
func TestResolveLabelPrefersMasterOverTranslations(t *testing.T) {
	root := t.TempDir()
	write := func(rel, contents string) {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("sfdx-project.json", `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"62.0"}`)
	write("force-app/main/default/labels/CustomLabels.labels-meta.xml", `<?xml version="1.0" encoding="UTF-8"?>
<CustomLabels xmlns="http://soap.sforce.com/2006/04/metadata">
    <labels>
        <fullName>Find_Button</fullName>
        <language>en_US</language>
        <protected>false</protected>
        <shortDescription>Find Button</shortDescription>
        <value>Find</value>
    </labels>
</CustomLabels>
`)
	for _, language := range []string{"da", "sv"} {
		write("force-app/main/default/translations/"+language+".translation-meta.xml", `<?xml version="1.0" encoding="UTF-8"?>
<Translations xmlns="http://soap.sforce.com/2006/04/metadata">
    <customLabels>
        <label><!-- Find Button --></label>
        <name>Find_Button</name>
    </customLabels>
</Translations>
`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := LoadProjectWithDependencies(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, namespace := range []string{"c", ""} {
		got, status := ResolveLabel(registry, p.Namespace, namespace, "Find_Button")
		if status != LabelLookupResolved || got != "Find" {
			t.Fatalf("namespace %q: got %q status=%s, want master value Find", namespace, got, status)
		}
	}
}
