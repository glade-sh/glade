package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Individual native rows: P0-new-objects/org.tsv and P0-expanded/org.tsv.
// The mixed-version sequence is a local cache-isolation control.
func TestStandardAPI68DescribeWarmVersionAndRoute(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
	for _, name := range []string{"AssociatedLocationHistory", "AssetStatePeriod", "WorkloadUnit"} {
		storage.EnsureStandardObject(&org, name)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	for _, row := range []struct {
		api                             string
		named, object, field, namedOnly bool
	}{
		{"62.0", false, false, false, false},
		{"68.0", false, true, true, false},
		{"68.0", true, true, true, true},
		{"67.0", true, false, false, false},
		{"68.0", false, true, true, false},
	} {
		machine.currentMethod = Method{APIVersion: row.api, SourceContextBound: true}
		machine.currentClass = ""
		if row.named {
			machine.currentClass = "CapturedNamedClass"
		}
		global := machine.globalDescribeMapValue()
		_, object := global.Map[mapKey(String(strings.ToLower("AssociatedLocationHistory")))]
		if object != row.object {
			t.Fatalf("api=%s named=%t: global object=%t, want %t", row.api, row.named, object, row.object)
		}
		asset := fieldMapGet(t, machine, fieldMapWrapper(t, machine, "AssetStatePeriod"))
		_, field := asset.Map[mapKey(String(strings.ToLower("RampUpliftType")))]
		if field != row.field {
			t.Fatalf("api=%s named=%t: field=%t, want %t", row.api, row.named, field, row.field)
		}
		work := fieldMapGet(t, machine, fieldMapWrapper(t, machine, "WorkloadUnit"))
		_, namedOnly := work.Map[mapKey(String(strings.ToLower("IntradayActualVolume")))]
		if namedOnly != row.namedOnly {
			t.Fatalf("api=%s named=%t: named-only field=%t, want %t", row.api, row.named, namedOnly, row.namedOnly)
		}
	}
}
