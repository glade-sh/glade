package sema

import (
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"testing"
)

func TestQualifiedReceiverRestoredAnonymousCompatibilityControls(t *testing.T) {
	const dir = "testdata/qualified_receiver_widening"
	index := receiverWideningIndex(t, dir)
	for _, tc := range []struct {
		name     string
		source   string
		rejected bool
	}{
		{name: "custom_field_token", source: "custom_field_token.apex"},
		{name: "namespaced_custom_field_token", source: "namespaced_custom_field_token.apex"},
		// Native receiver_member_routes/org.tsv:86-89,94-97 rejects this
		// receiver; the release kept the rejection and so does Glade.
		{name: "package_static_instance_property", source: "package_static_instance_property.apex", rejected: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join(dir, tc.source))
			if err != nil {
				t.Fatal(err)
			}
			result := AnalyzeAnonymous(index, string(source), "67.0")
			if result.HasErrors() != tc.rejected {
				t.Fatalf("receiver rejected=%v, want %v: %+v", result.HasErrors(), tc.rejected, result.Diagnostics)
			}
		})
	}
}

func receiverWideningIndex(t *testing.T, dir string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	rollupSource, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default/classes/Rollup.cls"))
	if err != nil {
		t.Fatal(err)
	}
	rollupClass := filepath.Join(root, "Rollup.cls")
	writeSemaFile(t, rollupClass, string(rollupSource))
	gatewaySource, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default/classes/GatewayService.cls"))
	if err != nil {
		t.Fatal(err)
	}
	gatewayClass := filepath.Join(root, "GatewayService.cls")
	writeSemaFile(t, gatewayClass, string(gatewaySource))
	index := typesys.Build(project.Project{
		Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{rollupClass, gatewayClass},
	}, schema.Schema{Objects: []schema.Object{
		{Name: "Account"},
		{Name: "Contact"},
		{Name: "Thing__c", Fields: []schema.Field{{Name: "Val__c", Type: "String"}}},
		{Name: "ns__Thing__c", Fields: []schema.Field{{Name: "ns__Val__c", Type: "String"}}},
		{Name: "Settings__c", CustomSettingsType: "List", Fields: []schema.Field{{Name: "Enabled__c", Type: "Checkbox"}}},
	}})
	if len(index.Diagnostics) != 0 {
		t.Fatalf("synthetic receiver-shape index diagnostics: %#v", index.Diagnostics)
	}
	index.VisualforcePagesKnown = true
	index.VisualforcePageNames = []string{"ownedlanding"}
	return index
}
