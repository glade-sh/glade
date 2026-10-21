package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned GM01-GM05 named API62/67/68 observations distinguish setup-only
// membership work, mixed account work, and the existing runAs exception.
func TestExecMixedDMLGroupMemberClassification(t *testing.T) {
	const group = `Group team = new Group(Name='Owned team', DeveloperName='OwnedTeam', Type='Regular'); insert team;`
	const member = `insert new GroupMember(GroupId=team.Id, UserOrGroupId=UserInfo.getUserId());`
	const role = `insert new UserRole(Name='Owned role', DeveloperName='OwnedRole');`
	for _, tc := range []struct{ name, source string }{
		{"role then member", role + group + member + `System.assertEquals(1, [SELECT COUNT() FROM GroupMember WHERE GroupId=:team.Id]);`},
		{"member then role", group + member + role + `System.assertEquals(1, [SELECT COUNT() FROM GroupMember WHERE GroupId=:team.Id]);`},
		{"member then account rejects", group + member + `
Boolean rejected=false;
try { insert new Account(Name='Owned customer'); } catch (DmlException e) { rejected=true; }
System.assert(rejected, 'membership DML is setup work');`},
		{"account then member rejects", `insert new Account(Name='Owned customer');` + group + `
Boolean rejected=false;
try { ` + member + ` } catch (DmlException e) { rejected=true; }
System.assert(rejected, 'membership DML is setup work');`},
		{"runAs control", `insert new Account(Name='Owned customer');` + group + `
System.runAs(new User(Id=UserInfo.getUserId())) { ` + member + ` }
System.assertEquals(1, [SELECT COUNT() FROM GroupMember WHERE GroupId=:team.Id]);`},
		{"mixed status code", group + member + `
Boolean rejected=false;
try { insert new Account(Name='Owned customer'); }
catch (DmlException e) {
    rejected=true;
    System.assertEquals(StatusCode.MIXED_DML_OPERATION, e.getDmlType(0));
}

System.assert(rejected);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "62.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			for _, object := range []string{"UserRole", "Group", "GroupMember"} {
				storage.EnsureStandardObject(&org, object)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			machine.EnableTestContext()
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMixedDMLRejectionExposesObservedStatus(t *testing.T) {
	machine := New(nil)
	machine.EnableTestContext()
	machine.testContext.SetupDML = true
	err := machine.checkMixedDML("insert", []storage.Record{{Object: "Account"}})
	thrown, ok := err.(*apexThrowError)
	if !ok {
		t.Fatalf("expected DmlException, got %v", err)
	}
	detail, err := dmlExceptionDetail(thrown.value, "getDmlType", []Value{Int(0)})
	if err != nil {
		t.Fatal(err)
	}
	if got := detail.Fields["statusCode"].String(); got != "MIXED_DML_OPERATION" {
		t.Fatalf("status=%s", got)
	}
}

func TestMixedDMLUnobservedErrorDetailsRetainBoundary(t *testing.T) {
	for _, tc := range []struct {
		operation string
		records   []storage.Record
	}{
		{"insert", []storage.Record{{Object: "Account"}, {Object: "Account"}}},
		{"update", []storage.Record{{Object: "Account"}}},
		{"delete", []storage.Record{{Object: "Account"}}},
	} {
		t.Run(tc.operation, func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			machine.testContext.SetupDML = true
			err := machine.checkMixedDML(tc.operation, tc.records)
			thrown, ok := err.(*apexThrowError)
			if !ok {
				t.Fatalf("existing rejection changed: %v", err)
			}
			if _, added := thrown.value.Fields["__dmlErrors"]; added {
				t.Fatal("unobserved operation/bulk details changed")
			}
		})
	}
}

func TestMixedDMLGroupMemberOtherOperationsRetainBoundary(t *testing.T) {
	// Preserve checks outside the captured insert/delete paths. The native
	// membership delete conformance row covers the delete exception separately.
	for _, operation := range []string{"update", "upsert", "undelete", "merge", "emptyRecycleBin"} {
		t.Run(operation, func(t *testing.T) {
			machine := New(nil)
			machine.EnableTestContext()
			machine.testContext.SetupDML = true
			if err := machine.checkMixedDML(operation, []storage.Record{{Object: "GroupMember"}}); err == nil || !strings.Contains(err.Error(), "Mixed DML") {
				t.Fatalf("existing boundary changed for %s: %v", operation, err)
			}
		})
	}
}
