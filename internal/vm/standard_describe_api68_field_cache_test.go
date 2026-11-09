package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// P0-expanded/org.tsv and P0-new-objects/org.tsv show these fields only on the named route.
func TestStandardAPI68NamedFieldDescribeCacheIsolation(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
	storage.EnsureStandardObject(&org, "WorkloadUnit")
	machine := New(nil)
	machine.SetOrg(&org)
	machine.currentMethod = Method{APIVersion: "68.0", SourceContextBound: true}
	for _, row := range []struct{ object, field string }{{"WorkloadUnit", "IntradayActualVolume"}, {"ShiftAllocationRequest", "CurrentAssigneeId"}} {
		t.Run(row.object, func(t *testing.T) {
			machine.currentClass = "CapturedNamedClass"
			if _, err := machine.describeFieldValue(row.object, row.field); err != nil {
				t.Fatalf("named field unavailable: %v", err)
			}
			machine.currentClass = ""
			if _, found := machine.lookupFieldDescribeCache(row.object, row.field); found {
				t.Fatalf("anonymous field unexpectedly inherited cached named describe")
			}
		})
	}
}
