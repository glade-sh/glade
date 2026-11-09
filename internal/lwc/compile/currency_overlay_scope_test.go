package compile

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A currency fix must not change admission based on an unrelated org feature.
func TestCurrencyOverlayLeavesUnrelatedFeatureShapeUnchanged(t *testing.T) {
	load := func(t *testing.T, features string) *salesforceImportConfig {
		t.Helper()
		root := t.TempDir()
		writeCompileFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
		writeCompileFixtureFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":`+features+`}`)
		p, err := project.Load(root)
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := loadSalesforceImports(p, []string{"@salesforce/schema/Account.Name"})
		if err != nil {
			t.Fatal(err)
		}
		return cfg
	}
	plain := load(t, `[]`)
	unrelated := load(t, `["PersonAccounts"]`)
	if !reflect.DeepEqual(plain.Schema, unrelated.Schema) {
		t.Fatal("currency import handling changed the schema for an unrelated feature")
	}
}
