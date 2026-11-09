package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Captured new-object describe membership does not establish DML or SOQL shape.
func TestStandardAPI68DescribeDoesNotExpandGeneralObjectResolution(t *testing.T) {
	const object = "ShiftAllocationRequest"
	if _, ok := storage.StandardObjectDefinition(object); ok {
		t.Fatal("test requires an object absent from the preexisting catalog")
	}
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "68.0", SourceContextBound: true}
	machine.currentClass = "CapturedNamedClass"
	if _, _, ok := machine.describeObjectDefinition(object); ok {
		t.Fatal("describe-only object entered shared object resolution")
	}
	if _, err := machine.describeFromSObjectTypeToken(sObjectTypeToken(object)); err != nil {
		t.Fatalf("captured describe route unavailable: %v", err)
	}
}
