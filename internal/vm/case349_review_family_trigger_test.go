package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestCase349BeforeTriggerBusinessFamilySignals(t *testing.T) {
	org := testDataOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	machine := New(nil)
	machine.SetOrg(&org)
	var business storage.ID
	for _, rt := range org.Objects["Account"].Definition.RecordTypes {
		if !rt.PersonAccount {
			business = rt.ID
			break
		}
	}
	if business == "" {
		t.Fatal("missing business type")
	}
	for _, field := range []string{"FirstName", "LastName"} {
		t.Run(field, func(t *testing.T) {
			rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(business), field: storage.StringValue("Synthetic")}}
			applyAccountBeforeDMLDerivedFields(machine, &rec)
			if value, ok := rec.GetField("IsPersonAccount"); ok && value.Kind == storage.ValueBoolean && value.Boolean {
				t.Error("explicit business type became person")
			}
		})
	}
}

func TestCase349SameValueTriggerAssignmentHydration(t *testing.T) {
	for _, field := range []string{"Name", "RecordTypeId"} {
		t.Run(field, func(t *testing.T) {
			org := testDataOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			machine := New(nil)
			machine.SetOrg(&org)
			program, err := CompileAnonymous("Trigger.new[0]." + field + "=Trigger.new[0]." + field + ";")
			if err != nil {
				t.Fatal(err)
			}
			before := []storage.Record{{Object: "Account", ID: "001000000000101", Fields: map[string]storage.Value{"IsPersonAccount": storage.BooleanValue(true), "Name": storage.StringValue("Synthetic"), "RecordTypeId": storage.IDValue("012000000000101")}}}
			records := []storage.Record{before[0]}
			trigger := Trigger{Name: "SyntheticObserver", Object: "Account", Timing: "before", Operation: "update", Program: program}
			if _, err = machine.runTrigger(trigger, records, before, &Result{}); err != nil {
				t.Fatal(err)
			}
			original := []storage.Record{{Object: "Account", ID: before[0].ID}}
			removeUnchangedHydratedPersonFields(records, original, before)
			if _, ok := records[0].GetField(field); ok != (field == "RecordTypeId") {
				t.Errorf("trigger assignment presence for %s = %v", field, ok)
			}
		})
	}
}

func TestCase349PreflightUsesExplicitBusinessFamily(t *testing.T) {
	for _, supplied := range []bool{false, true} {
		t.Run(map[bool]string{false: "sparse", true: "selected"}[supplied], func(t *testing.T) {
			org := testDataOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			business := storage.ID("012000000000101")
			id := storage.ID("001000000000101")
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{{ID: business, DeveloperName: "Synthetic", Active: true, Available: true}}
			if account.Records == nil {
				account.Records = map[storage.ID]storage.Record{}
			}
			account.Records[id] = storage.Record{Object: "Account", ID: id, Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(business), "IsPersonAccount": storage.BooleanValue(true)}}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			fields := map[string]storage.Value{"FirstName": storage.StringValue("Synthetic")}
			if supplied {
				fields["RecordTypeId"] = storage.IDValue(business)
			}
			failures := machine.preflightBusinessAccountNameFailures([]storage.Record{{Object: "Account", ID: id, Fields: fields}}, "update", "")
			if len(failures) != 1 || failures[0].Success {
				t.Errorf("explicit business metadata ignored: %#v", failures)
			}
		})
	}
}
