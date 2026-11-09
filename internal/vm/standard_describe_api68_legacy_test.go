package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// A persisted pre-provenance schema cannot establish whether an old field was
// explicitly configured, so the existing field must survive refresh.
func TestStandardAPI68NewObjectPreservesLegacyFieldWithoutProvenance(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
	storage.EnsureStandardObject(&org, "AssociatedLocationHistory")
	state := org.Objects["AssociatedLocationHistory"]
	delete(state.Definition.Metadata, "__glade_standard_configured_policy_fields")
	org.Objects["AssociatedLocationHistory"] = state
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "68.0", SourceContextBound: true}
	fields := fieldMapGet(t, machine, fieldMapAPI68Wrapper(t, machine, "AssociatedLocationHistory"))
	if _, ok := fields.Map[mapKey(String(strings.ToLower("Division")))]; !ok {
		t.Fatal("legacy field was dropped without provenance")
	}
}
