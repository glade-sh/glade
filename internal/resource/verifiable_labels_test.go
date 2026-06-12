package resource

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestResolvePackageLabelCPrefix(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "lightning-out-vf")
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
		{name: "Greeting", value: "Hello from Glade"},
	} {
		got, status := ResolveLabel(registry, p.Namespace, "c", tc.name)
		if status != LabelLookupResolved || got != tc.value {
			t.Fatalf("%s = %q status=%s want %q", tc.name, got, status, tc.value)
		}
	}
}
