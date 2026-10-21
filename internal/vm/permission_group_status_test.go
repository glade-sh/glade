package vm

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestPermissionSetGroupInsertedStatusIsStoredOnly(t *testing.T) {
	machine := permissionGroupStatusMachine(t, false)
	program, err := CompileAnonymous(`
PermissionSetGroup groupRow=new PermissionSetGroup(DeveloperName='OwnedEmptyGroup',MasterLabel='Distinct empty group');
insert groupRow;
System.assertEquals(null,groupRow.Status);
System.assertEquals('Updated',[SELECT Status FROM PermissionSetGroup WHERE Id=:groupRow.Id].Status);
PermissionSet emptySet=new PermissionSet(Name='OwnedEmptySet',Label='Distinct empty set');insert emptySet;
insert new PermissionSetGroupComponent(PermissionSetGroupId=groupRow.Id,PermissionSetId=emptySet.Id);
System.assertEquals(null,groupRow.Status);
System.assertEquals('Outdated',[SELECT Status FROM PermissionSetGroup WHERE Id=:groupRow.Id].Status);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionSetGroupCalculationCountersAndRollback(t *testing.T) {
	for _, journal := range []bool{false, true} {
		t.Run(map[bool]string{false: "snapshot", true: "journal"}[journal], func(t *testing.T) {
			machine := permissionGroupStatusMachine(t, journal)
			program, err := CompileAnonymous(`
PermissionSetGroup first=new PermissionSetGroup(DeveloperName='OwnedFirstGroup',MasterLabel='First group label');
PermissionSetGroup second=new PermissionSetGroup(DeveloperName='OwnedSecondGroup',MasterLabel='Second group label');
insert new List<PermissionSetGroup>{first,second};
PermissionSet emptySet=new PermissionSet(Name='OwnedEmptySet',Label='Distinct empty set');insert emptySet;
insert new List<PermissionSetGroupComponent>{new PermissionSetGroupComponent(PermissionSetGroupId=first.Id,PermissionSetId=emptySet.Id),new PermissionSetGroupComponent(PermissionSetGroupId=second.Id,PermissionSetId=emptySet.Id)};
Savepoint point=Database.setSavepoint();
Integer statements=Limits.getDMLStatements(),rows=Limits.getDMLRows(),queries=Limits.getQueries();
Test.calculatePermissionSetGroup(new List<Id>{first.Id,second.Id});
System.assertEquals(statements+1,Limits.getDMLStatements());
System.assertEquals(rows+2,Limits.getDMLRows());
System.assertEquals(queries,Limits.getQueries());
System.assertEquals('Updated',[SELECT Status FROM PermissionSetGroup WHERE Id=:first.Id].Status);
System.assertEquals('Updated',[SELECT Status FROM PermissionSetGroup WHERE Id=:second.Id].Status);
System.assertEquals(null,first.Status);
statements=Limits.getDMLStatements();rows=Limits.getDMLRows();
Test.calculatePermissionSetGroup(first.Id);
System.assertEquals(statements,Limits.getDMLStatements());System.assertEquals(rows,Limits.getDMLRows());
Database.rollback(point);
System.assertEquals('Outdated',[SELECT Status FROM PermissionSetGroup WHERE Id=:first.Id].Status);
System.assertEquals('Outdated',[SELECT Status FROM PermissionSetGroup WHERE Id=:second.Id].Status);
`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPermissionSetGroupCalculationLimitFailureDoesNotChangeStatus(t *testing.T) {
	machine := permissionGroupStatusMachine(t, true)
	engine := dml.NewEngine(machine.Org)
	inserted := engine.Insert([]storage.Record{{Object: "PermissionSetGroup", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("OwnedPendingGroup"), "MasterLabel": storage.StringValue("Pending group label")}}})
	if len(inserted) != 1 || !inserted[0].Success {
		t.Fatalf("group setup=%#v", inserted)
	}
	group := machine.Org.Objects["PermissionSetGroup"]
	record := group.Records[inserted[0].ID].Clone()
	record.Fields["Status"] = storage.StringValue("Outdated")
	group.Records[record.ID] = record
	machine.Org.Objects["PermissionSetGroup"] = group
	caps := machine.limitCaps
	caps.DMLStatements = 0
	machine.SetLimitCaps(caps)
	machine.SetLimitMode(LimitModeStrict)
	program, err := CompileAnonymous(fmt.Sprintf("Test.calculatePermissionSetGroup('%s');", record.ID))
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.LimitException" {
		t.Fatalf("calculation error=%v, want LimitException", err)
	}
	if got := machine.Org.Objects["PermissionSetGroup"].Records[record.ID].Fields["Status"].String; got != "Outdated" {
		t.Fatalf("status after rejected calculation=%q", got)
	}
}

func permissionGroupStatusMachine(t *testing.T, journal bool) *VM {
	t.Helper()
	org := testDataOrg()
	storage.EnsureDeterministicPlatformData(&org)
	for _, name := range []string{"PermissionSetGroup", "PermissionSetGroupComponent", "PermissionSet"} {
		storage.EnsureStandardObject(&org, name)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if journal {
		machine.SetIsolationJournal(storage.NewIsolationJournal(machine.Org))
	}
	return machine
}

// Preserve the prior no-op for argument shapes outside the native packet.
func TestPermissionSetGroupCalculationPreservesUnobservedListShapes(t *testing.T) {
	for _, kind := range []string{"null element", "missing group", "duplicate group"} {
		t.Run(kind, func(t *testing.T) {
			machine := permissionGroupStatusMachine(t, false)
			engine := dml.NewEngine(machine.Org)
			inserted := engine.Insert([]storage.Record{{Object: "PermissionSetGroup", Fields: map[string]storage.Value{"DeveloperName": storage.StringValue("OwnedPendingGroup"), "MasterLabel": storage.StringValue("Pending group label")}}})
			if len(inserted) != 1 || !inserted[0].Success {
				t.Fatalf("group setup=%#v", inserted)
			}
			object := machine.Org.Objects["PermissionSetGroup"]
			record := object.Records[inserted[0].ID].Clone()
			record.Fields["Status"] = storage.StringValue("Outdated")
			object.Records[record.ID] = record
			machine.Org.Objects["PermissionSetGroup"] = object
			other := "null"
			if kind == "missing group" {
				other = "'0PG000000000999'"
			}
			if kind == "duplicate group" {
				other = fmt.Sprintf("'%s'", record.ID)
			}
			program, err := CompileAnonymous(fmt.Sprintf("Test.calculatePermissionSetGroup(new List<Id>{'%s',%s});", record.ID, other))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			if got := machine.Org.Objects["PermissionSetGroup"].Records[record.ID].Fields["Status"].String; got != "Outdated" {
				t.Fatalf("unobserved shape changed status=%q", got)
			}
			if machine.limits.DMLStatements != 0 || machine.limits.DMLRows != 0 {
				t.Fatal("unobserved shape charged DML")
			}
		})
	}
}
