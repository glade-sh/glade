package vm_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestAssignmentRuleHeaderUnknownIDAnonymousAndIsTestAPIRoutes(t *testing.T) {
	// The native assignment_rules A05 row accepts an unknown rule ID and keeps
	// the Lead with its ordinary user owner on both entry routes and APIs.
	const source = `
Lead row = new Lead(LastName='Synthetic', Company='route-match');
Database.DMLOptions options = new Database.DMLOptions();
options.assignmentRuleHeader.assignmentRuleId = '01Q000000000001';
Database.SaveResult saved = Database.insert(row, options);
System.assertEquals(true, saved.isSuccess());
row = [SELECT OwnerId FROM Lead WHERE Id = :saved.Id];
System.assertEquals(false, String.valueOf(row.OwnerId).startsWith('00G'));
`
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(source, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "Lead")
				machine := vm.New(nil)
				machine.SetOrg(&org)
				if route == "istest" {
					machine.EnableTestContext()
					if err := machine.RegisterMethod(vm.Method{Name: "AssignmentHeaderTest.observe", ClassName: "AssignmentHeaderTest", IsStatic: true, Access: "public", Modifiers: []string{"isTest"}, APIVersion: api, Program: program}); err != nil {
						t.Fatal(err)
					}
					invoke, compileErr := vm.CompileAnonymousWithOptions("AssignmentHeaderTest.observe();", vm.CompileOptions{APIVersion: api})
					if compileErr != nil {
						t.Fatal(compileErr)
					}
					_, err = machine.ExecuteTestInClass(invoke, "AssignmentHeaderTest")
				} else {
					_, err = machine.Execute(program)
				}
				if err != nil {
					t.Fatalf("native unknown-rule insert succeeds with user ownership: %v", err)
				}
			})
		}
	}
}
