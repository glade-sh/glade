package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Local configured metadata is positive evidence independent of the scratch map.
func TestStandardAPI68NewObjectKeepsConfiguredField(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
	storage.EnsureStandardObject(&org, "AssociatedLocationHistory")
	state := org.Objects["AssociatedLocationHistory"]
	state.Definition.Fields["SyntheticExtra__c"] = storage.Field{APIName: "SyntheticExtra__c", Label: "Synthetic Extra", Type: storage.FieldString}
	org.Objects["AssociatedLocationHistory"] = state
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "68.0", SourceContextBound: true}
	value := fieldMapGet(t, machine, fieldMapAPI68Wrapper(t, machine, "AssociatedLocationHistory"))
	if _, ok := value.Map[mapKey(String(strings.ToLower("SyntheticExtra__c")))]; !ok {
		t.Fatal("configured field was removed")
	}
}
