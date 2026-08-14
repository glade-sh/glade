package vm

import (
	"io"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/storage"
)

func TestCurrentSourceExecutionPolicyUsesAPIVersionAndTriggerFrame(t *testing.T) {
	machine := New(io.Discard)
	machine.currentClass = "Probe"
	machine.currentMethod = Method{APIVersion: "67.0"}

	policy := machine.currentSourceExecutionPolicy()
	if policy.DataMode != "USER_MODE" || policy.SharingMode != "with sharing" || policy.Trigger {
		t.Fatalf("API 67 default policy = %#v", policy)
	}

	machine.currentMethod.APIVersion = "66.0"
	policy = machine.currentSourceExecutionPolicy()
	if policy.DataMode != "SYSTEM_MODE" || policy.SharingMode != "without sharing" {
		t.Fatalf("API 66 default policy = %#v", policy)
	}

	machine.currentMethod.APIVersion = "67.0"
	machine.currentTrigger = true
	policy = machine.currentSourceExecutionPolicy()
	if policy.DataMode != "USER_MODE" || policy.SharingMode != "without sharing" || !policy.Trigger {
		t.Fatalf("trigger policy = %#v", policy)
	}
}

func TestCurrentSourceExecutionPolicyHonorsSharingModifiers(t *testing.T) {
	machine := New(io.Discard)
	machine.currentMethod = Method{APIVersion: "67.0"}
	machine.Classes["ExplicitWithout"] = Class{Name: "ExplicitWithout", Modifiers: []string{"without sharing"}}
	machine.currentClass = "ExplicitWithout"
	if got := machine.currentSourceExecutionPolicy().SharingMode; got != "without sharing" {
		t.Fatalf("explicit without sharing = %q", got)
	}

	machine.Classes["Inherited"] = Class{Name: "Inherited", Modifiers: []string{"inherited sharing"}}
	machine.currentClass = "Inherited"
	machine.callStack = []callFrame{{Symbol: "ExplicitWithout.run"}}
	if got := machine.currentSourceExecutionPolicy().SharingMode; got != "without sharing" {
		t.Fatalf("inherited sharing = %q", got)
	}

	machine.Classes["LegacyImplicit"] = Class{Name: "LegacyImplicit"}
	machine.currentClass = "LegacyImplicit"
	machine.currentMethod.APIVersion = "66.0"
	machine.callStack = []callFrame{{Symbol: "ExplicitWith.run"}}
	machine.Classes["ExplicitWith"] = Class{Name: "ExplicitWith", Modifiers: []string{"with sharing"}}
	if got := machine.currentSourceExecutionPolicy().SharingMode; got != "with sharing" {
		t.Fatalf("legacy implicit caller sharing = %q", got)
	}

	machine.currentMethod.APIVersion = "67.0"
	machine.callStack = []callFrame{{Symbol: "ExplicitWithout.run"}}
	if got := machine.currentSourceExecutionPolicy().SharingMode; got != "with sharing" {
		t.Fatalf("API 67 implicit default = %q", got)
	}
}

func TestResolveDMLModeUsesExplicitModeBeforeSourceDefault(t *testing.T) {
	machine := New(io.Discard)
	machine.currentMethod = Method{APIVersion: "67.0"}
	if got := machine.resolveDMLMode(ir.DMLModeDefault); got != "USER_MODE" {
		t.Fatalf("default API 67 DML mode = %q", got)
	}
	if got := machine.resolveDMLMode(ir.DMLModeSystem); got != "SYSTEM_MODE" {
		t.Fatalf("explicit system DML mode = %q", got)
	}
	if got := machine.resolveDMLMode(ir.DMLModeUser); got != "USER_MODE" {
		t.Fatalf("explicit user DML mode = %q", got)
	}
}

func TestAPI67OmittedDatabaseOperationsUseUserMode(t *testing.T) {
	modernSOQL := New(io.Discard)
	modernSOQL.currentMethod = Method{APIVersion: "67.0"}
	modernSOQLOrg := orgForSecurePolicyTest()
	modernSOQL.SetOrg(&modernSOQLOrg)
	modernSOQL.executionUser = stripInaccessibleTestUser()
	if _, err := modernSOQL.executeSOQL("SELECT Id, Secret__c FROM Account", &Result{}); err == nil {
		t.Fatal("API 67 omitted SOQL unexpectedly bypassed field permissions")
	}
	if _, err := modernSOQL.executeSOQL("SELECT Id, Secret__c FROM Account WITH SYSTEM_MODE", &Result{}); err != nil {
		t.Fatalf("API 67 explicit system SOQL = %v", err)
	}

	legacySOQL := New(io.Discard)
	legacySOQL.currentMethod = Method{APIVersion: "66.0"}
	legacySOQLOrg := orgForSecurePolicyTest()
	legacySOQL.SetOrg(&legacySOQLOrg)
	legacySOQL.executionUser = stripInaccessibleTestUser()
	if _, err := legacySOQL.executeSOQL("SELECT Id, Secret__c FROM Account", &Result{}); err != nil {
		t.Fatalf("API 66 omitted SOQL = %v", err)
	}

	program, err := CompileAnonymous(`
Account row = new Account(Id = '001000000000001', Secret__c = 'changed');
update row;
`)
	if err != nil {
		t.Fatal(err)
	}

	modern := New(io.Discard)
	modern.currentMethod = Method{APIVersion: "67.0"}
	modernOrg := orgForSecurePolicyTest()
	modern.SetOrg(&modernOrg)
	modern.executionUser = stripInaccessibleTestUser()
	if _, err := modern.Execute(program); err == nil {
		t.Fatal("API 67 omitted DML unexpectedly bypassed field permissions")
	}

	systemProgram, err := CompileAnonymous(`
Account row = new Account(Id = '001000000000001', Secret__c = 'changed');
update as system row;
`)
	if err != nil {
		t.Fatal(err)
	}
	systemMachine := New(io.Discard)
	systemMachine.currentMethod = Method{APIVersion: "67.0"}
	systemOrg := orgForSecurePolicyTest()
	systemMachine.SetOrg(&systemOrg)
	systemMachine.executionUser = stripInaccessibleTestUser()
	if _, err := systemMachine.Execute(systemProgram); err != nil {
		t.Fatalf("API 67 explicit system DML = %v", err)
	}

	legacy := New(io.Discard)
	legacy.currentMethod = Method{APIVersion: "66.0"}
	legacyOrg := orgForSecurePolicyTest()
	legacy.SetOrg(&legacyOrg)
	legacy.executionUser = stripInaccessibleTestUser()
	if _, err := legacy.Execute(program); err != nil {
		t.Fatalf("API 66 omitted DML = %v", err)
	}
}

func orgForSecurePolicyTest() storage.OrgState {
	return stripInaccessibleTestOrg()
}
