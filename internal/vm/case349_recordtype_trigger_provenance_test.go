package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Trigger round-trips preserve VM default provenance unless the field is assigned.
func TestCase349VMDefaultRecordTypeTriggerProvenance38b(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		want         bool
	}{{"untouched", "Integer observed=1;", true}, {"explicit-same", "Trigger.new[0].RecordTypeId=Trigger.new[0].RecordTypeId;", false}} {
		t.Run(tc.name, func(t *testing.T) {
			org := testDataOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymous(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			rec := storage.Record{Object: "Account", ID: "001000000000101", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000101")}}
			rec.MarkDMLRecordTypeDefaulted()
			rows := []storage.Record{rec}
			trigger := Trigger{Name: "SyntheticObserver", Object: "Account", Timing: "before", Operation: "update", Program: program}
			if _, err = machine.runTrigger(trigger, rows, nil, &Result{}); err != nil {
				t.Fatal(err)
			}
			if got := rows[0].DMLRecordTypeDefaulted(); got != tc.want {
				t.Errorf("default provenance=%v want %v", got, tc.want)
			}
		})
	}
}
