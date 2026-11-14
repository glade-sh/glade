package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestChildRelationshipParentFallbackDoesNotInstall(t *testing.T) {
	org := childRelationshipLookupOrg(nil, "")
	parent := ""
	for _, name := range storage.KnownStandardObjectNames() {
		if _, owed := storage.LazyStandardObjectName(&org, name); owed {
			parent = " " + name + " "
			break
		}
	}
	if parent == "" {
		t.Fatal("no pending catalog object")
	}
	baseline := org.CloneRuntimeFrozenDefinition()
	want, ok := storage.ResolveObjectName(baseline, parent)
	if !ok {
		want = parent
	}
	before := len(org.Objects)
	got := resolvedChildRelationshipParentName(org, map[string]string{}, parent)
	if got != want {
		t.Fatalf("fallback returned %q, resolver returned %q", got, want)
	}
	if len(org.Objects) != before {
		t.Fatalf("fallback installed objects: %d before, %d after", before, len(org.Objects))
	}
}
