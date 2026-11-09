package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Explicit project fields remain positive evidence even if their metadata
// happens to equal the generated baseline field that the scratch map omits.
func TestStandardAPI68NewObjectKeepsIdenticalConfiguredField(t *testing.T) {
	const object = "AssociatedLocationHistory"
	baseline, ok := storage.StandardObjectDefinition(object)
	if !ok {
		t.Fatal("missing historical baseline")
	}
	oldName, ok := storage.ResolveFieldName(baseline, "", "Division")
	if !ok {
		t.Fatal("missing historical field")
	}
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
	storage.EnsureStandardObject(&org, object)
	configured := storage.ObjectDefinition{APIName: object, Fields: map[string]storage.Field{oldName: baseline.Fields[oldName]}}
	storage.EnsureStandardObjectFieldsForFeatures(&configured, org.Features)
	state := org.Objects[object]
	state.Definition = configured
	org.Objects[object] = state
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "68.0", SourceContextBound: true}
	fields := fieldMapGet(t, machine, fieldMapAPI68Wrapper(t, machine, object))
	if _, ok := fields.Map[mapKey(String(strings.ToLower(oldName)))]; !ok {
		t.Fatal("explicitly configured baseline-equal field was removed")
	}
}
