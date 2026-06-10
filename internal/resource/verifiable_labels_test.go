package resource

import (
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestResolveVerifiablePackageLabelCPrefix(t *testing.T) {
	root := "/Users/matt/.sf-repo-analysis/repos/sf-cred-pkg-develop"
	if _, err := os.Stat(root); err != nil {
		t.Skip("sf-cred-pkg-develop not present")
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := LoadProjectWithDependencies(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		value string
	}{
		{name: "connectVerifiableSync", value: "Connect Verifiable Sync"},
		{name: "connectVerifiableSyncDescription", value: "Receive updates in Salesforce by subscribing to Verifiable Sync."},
		{name: "cancel", value: "Cancel"},
		{name: "saveAndClose", value: "Save & Close"},
	} {
		got, status := ResolveLabel(registry, p.Namespace, "c", tc.name)
		if status != LabelLookupResolved || got != tc.value {
			t.Fatalf("%s = %q status=%s want %q", tc.name, got, status, tc.value)
		}
	}
}
