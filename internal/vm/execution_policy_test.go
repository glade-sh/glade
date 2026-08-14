package vm

import (
	"fmt"
	"io"
	"strings"
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

func TestDefaultSharingAndInheritedSharingResolution(t *testing.T) {
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

func TestSharingDefaultFollowsAPIVersionInheritanceChain(t *testing.T) {
	for _, test := range []struct {
		name        string
		parentAPI   string
		childAPI    string
		wantSharing string
	}{
		{name: "legacy chain", parentAPI: "66.0", childAPI: "66.0", wantSharing: "without sharing"},
		{name: "modern parent", parentAPI: "67.0", childAPI: "66.0", wantSharing: "with sharing"},
		{name: "modern child", parentAPI: "66.0", childAPI: "67.0", wantSharing: "with sharing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			machine := New(io.Discard)
			if err := machine.RegisterClass(Class{Name: "Parent", APIVersion: test.parentAPI}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{Name: "Child", APIVersion: test.childAPI, SuperClass: "Parent"}); err != nil {
				t.Fatal(err)
			}
			machine.currentClass = "Child"
			machine.currentMethod = Method{APIVersion: test.childAPI}
			if got := machine.currentSharingMode(); got != test.wantSharing {
				t.Fatalf("sharing = %q, want %q", got, test.wantSharing)
			}
		})
	}
}

func TestDuplicateClassMergeKeepsPreferredAPIVersion(t *testing.T) {
	merged := mergeDuplicateClass(
		Class{Name: "Shared", APIVersion: "66.0", Dependency: true},
		Class{Name: "Shared", APIVersion: "67.0"},
	)
	if merged.APIVersion != "67.0" {
		t.Fatalf("merged API version = %q, want 67.0", merged.APIVersion)
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

func TestDefaultUserModeAPI67OmittedDatabaseOperations(t *testing.T) {
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

func TestMixedVersionDatabaseOperationMode(t *testing.T) {
	program, err := CompileAnonymous("Account row = new Account(Id = '001000000000001', Secret__c = 'changed'); update row;")
	if err != nil {
		t.Fatal(err)
	}
	modern := New(io.Discard)
	modern.currentMethod = Method{APIVersion: "66.0"}
	modern.executionUser = stripInaccessibleTestUser()
	org := orgForSecurePolicyTest()
	modern.SetOrg(&org)
	modernMethod := Method{Name: "Modern.run", ClassName: "Modern", APIVersion: "67.0", Program: program}
	if _, err := modern.callMethod(modernMethod, nil, &Result{}); err == nil {
		t.Fatal("API 67 callee unexpectedly inherited API 66 system-mode default")
	}

	legacy := New(io.Discard)
	legacy.currentMethod = Method{APIVersion: "67.0"}
	legacy.executionUser = stripInaccessibleTestUser()
	org = orgForSecurePolicyTest()
	legacy.SetOrg(&org)
	legacyMethod := Method{Name: "Legacy.run", ClassName: "Legacy", APIVersion: "66.0", Program: program}
	if _, err := legacy.callMethod(legacyMethod, nil, &Result{}); err != nil {
		t.Fatalf("API 66 callee unexpectedly inherited API 67 user-mode default: %v", err)
	}
}

func TestTriggerUserModeStillEnforcesRecordVisibility(t *testing.T) {
	org := privateExecutionPolicyOrg()
	machine := New(io.Discard)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	machine.currentMethod = Method{APIVersion: "67.0"}
	machine.currentTrigger = true

	rows, err := machine.searchSuggestionRows("Other", "Widget__c", Null, Null)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.List) != 0 {
		t.Fatalf("trigger user-mode suggestions = %d, want 0", len(rows.List))
	}
}

func TestTriggerUserModePublicQueryAndSearchPaths(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
List<Widget__c> rows = [SELECT Id FROM Widget__c];
if (rows.size() != 1) { throw new DmlException('trigger SOQL exposed another owner'); }
List<Widget__c> databaseRows = Database.query('SELECT Id FROM Widget__c');
if (databaseRows.size() != 1 || Database.countQuery('SELECT Id FROM Widget__c') != 1) { throw new DmlException('trigger Database query exposed another owner'); }
Search.SearchResults found = Search.find('FIND {Other} IN ALL FIELDS RETURNING Widget__c(Id, Name)');
if (found.get('Widget__c').size() != 0) { throw new DmlException('trigger Search.find exposed another owner'); }
List<List<SObject>> queried = Search.query('FIND {Other} IN ALL FIELDS RETURNING Widget__c(Id, Name)');
if (queried[0].size() != 0) { throw new DmlException('trigger Search.query exposed another owner'); }
List<Widget__c> inlineRows = (List<Widget__c>)([FIND 'Other' IN ALL FIELDS RETURNING Widget__c(Id, Name)][0]);
if (inlineRows.size() != 0) { throw new DmlException('trigger inline SOSL exposed another owner'); }
Search.SuggestionOption option = new Search.SuggestionOption();
if (Search.suggest('Other', 'Widget__c', option).getSuggestionResults().size() != 0) { throw new DmlException('trigger Search.suggest exposed another owner'); }
`, CompileOptions{APIVersion: "67.0", Trigger: true})
	if err != nil {
		t.Fatal(err)
	}
	org := privateExecutionPolicyOrg()
	machine := New(io.Discard)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	if _, err := machine.runTrigger(Trigger{Name: "WidgetSecurityTrigger", Object: "Widget__c", Timing: "before", Operation: "insert", APIVersion: "67.0", Program: program}, nil, nil, &Result{}); err != nil {
		t.Fatal(err)
	}
}

func TestTriggerHandlerUsesOwnSharingContext(t *testing.T) {
	handlerProgram, err := CompileAnonymousWithOptions(`
List<Widget__c> rows = [SELECT Id FROM Widget__c];
if (rows.size() != 1) { throw new DmlException('handler sharing context was lost'); }
`, CompileOptions{APIVersion: "66.0"})
	if err != nil {
		t.Fatal(err)
	}
	triggerProgram, err := CompileAnonymousWithOptions("Handler.run();", CompileOptions{APIVersion: "67.0", Trigger: true})
	if err != nil {
		t.Fatal(err)
	}
	org := privateExecutionPolicyOrg()
	machine := New(io.Discard)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	if err := machine.RegisterClass(Class{Name: "Handler", APIVersion: "66.0", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterMethod(Method{Name: "Handler.run", ClassName: "Handler", IsStatic: true, APIVersion: "66.0", Program: handlerProgram}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.runTrigger(Trigger{Name: "WidgetHandlerTrigger", Object: "Widget__c", Timing: "before", Operation: "insert", APIVersion: "67.0", Program: triggerProgram}, nil, nil, &Result{}); err != nil {
		t.Fatal(err)
	}
}

func TestTriggerUserModePublicSOQLChecksFieldPermissions(t *testing.T) {
	program, err := CompileAnonymousWithOptions("List<Account> rows = [SELECT Id, Secret__c FROM Account];", CompileOptions{APIVersion: "67.0", Trigger: true})
	if err != nil {
		t.Fatal(err)
	}
	org := orgForSecurePolicyTest()
	machine := New(io.Discard)
	machine.SetOrg(&org)
	machine.executionUser = stripInaccessibleTestUser()
	if _, err := machine.runTrigger(Trigger{Name: "AccountSecurityTrigger", Object: "Account", Timing: "before", Operation: "insert", APIVersion: "67.0", Program: program}, nil, nil, &Result{}); err == nil {
		t.Fatal("trigger user-mode SOQL unexpectedly bypassed field permissions")
	}
}

func TestTriggerUserModePublicSOQLChecksObjectPermissions(t *testing.T) {
	program, err := CompileAnonymousWithOptions("List<Denied__c> rows = [SELECT Id FROM Denied__c];", CompileOptions{APIVersion: "67.0", Trigger: true})
	if err != nil {
		t.Fatal(err)
	}
	org := orgForSecurePolicyTest()
	org.Objects["Denied__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Denied__c", KeyPrefix: "a99", Fields: map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}}},
		Records:    map[storage.ID]storage.Record{},
	}
	machine := New(io.Discard)
	machine.SetOrg(&org)
	machine.executionUser = stripInaccessibleTestUser()
	if _, err := machine.runTrigger(Trigger{Name: "DeniedSecurityTrigger", Object: "Denied__c", Timing: "before", Operation: "insert", APIVersion: "67.0", Program: program}, nil, nil, &Result{}); err == nil {
		t.Fatal("trigger user-mode SOQL unexpectedly bypassed object permissions")
	}
}

func TestTriggerUserModePublicDMLChecksRecordPermissions(t *testing.T) {
	program, err := CompileAnonymousWithOptions("update Trigger.new;", CompileOptions{APIVersion: "67.0", Trigger: true})
	if err != nil {
		t.Fatal(err)
	}
	org := privateExecutionPolicyOrg()
	machine := New(io.Discard)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	other := org.Objects["Widget__c"].Records["a00000000000002"]
	failures, err := machine.runTrigger(Trigger{Name: "WidgetDMLSecurityTrigger", Object: "Widget__c", Timing: "before", Operation: "insert", APIVersion: "67.0", Program: program}, []storage.Record{other}, nil, &Result{})
	if err == nil && (len(failures) == 0 || failures[0].Success) {
		t.Fatal("trigger user-mode DML unexpectedly updated another owner's record")
	}
}

func TestTriggerUserModePublicDatabaseDMLChecksRecordPermissions(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
Boolean denied = false;
try {
	Database.update(Trigger.new);
} catch (DmlException e) {
	denied = true;
}
if (!denied) { throw new DmlException('trigger Database.update bypassed record permissions'); }
`, CompileOptions{APIVersion: "67.0", Trigger: true})
	if err != nil {
		t.Fatal(err)
	}
	org := privateExecutionPolicyOrg()
	machine := New(io.Discard)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	other := org.Objects["Widget__c"].Records["a00000000000002"]
	if _, err := machine.runTrigger(Trigger{Name: "WidgetDatabaseDMLSecurityTrigger", Object: "Widget__c", Timing: "before", Operation: "insert", APIVersion: "67.0", Program: program}, []storage.Record{other}, nil, &Result{}); err == nil || !strings.Contains(err.Error(), "Access to record") {
		t.Fatalf("trigger Database.update error = %v, want record access denial", err)
	}
}

func TestSystemModeDMLStillHonorsWithSharing(t *testing.T) {
	org := privateExecutionPolicyOrg()
	machine := New(io.Discard)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	machine.currentClass = "WithSharing"
	machine.currentMethod = Method{APIVersion: "67.0"}
	if err := machine.RegisterClass(Class{Name: "WithSharing", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}

	row := Object("Widget__c")
	row.Fields["Id"] = String("a00000000000002")
	if err := machine.enforceDMLRecordAccess("update", row, "", false); err == nil {
		t.Fatal("system-mode DML unexpectedly bypassed with-sharing record access")
	}
}

func TestUserModeUpsertChecksMatchedRecord(t *testing.T) {
	org := privateExecutionPolicyOrg()
	account := org.Objects["Widget__c"]
	account.Definition.Fields["External_Key__c"] = storage.Field{APIName: "External_Key__c", Type: storage.FieldString, ExternalID: true}
	account.Records["a00000000000002"].Fields["External_Key__c"] = storage.StringValue("other-key")
	org.Objects["Widget__c"] = account
	program, err := CompileAnonymous("Widget__c row = new Widget__c(Name = 'Changed', External_Key__c = 'other-key'); upsert row External_Key__c;")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(io.Discard)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
	machine.currentMethod = Method{APIVersion: "67.0"}
	if _, err := machine.Execute(program); err == nil {
		t.Fatal("user-mode upsert unexpectedly updated an inaccessible record")
	}
}

func TestMixedVersionSharingUsesResolvedCallerFrame(t *testing.T) {
	machine := New(io.Discard)
	machine.currentClass = "LegacyChild"
	machine.currentMethod = Method{APIVersion: "66.0"}
	machine.callStack = []callFrame{{Symbol: "ModernCaller.run", SharingMode: "with sharing"}}
	if got := machine.currentSharingMode(); got != "with sharing" {
		t.Fatalf("legacy callee sharing = %q, want with sharing", got)
	}
}

func TestDefaultSharingAuraAndLWCEntryPointsUseWithSharing(t *testing.T) {
	program, err := CompileAnonymous("return [SELECT Id FROM Widget__c].size();")
	if err != nil {
		t.Fatal(err)
	}
	for _, framework := range []string{"aura", "lwc"} {
		t.Run(framework, func(t *testing.T) {
			org := privateExecutionPolicyOrg()
			machine := New(io.Discard)
			machine.SetOrg(&org)
			machine.SetCurrentUser(storage.Record{ID: "005000000000001", Object: "User"})
			if err := machine.RegisterClass(Class{Name: "EntryProbe"}); err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterMethod(Method{Name: "EntryProbe.run", ClassName: "EntryProbe", IsStatic: true, Access: "public", Modifiers: []string{"AuraEnabled"}, APIVersion: "66.0", Program: program}); err != nil {
				t.Fatal(err)
			}
			var invocation UIInvocationResult
			if framework == "aura" {
				invocation, err = machine.InvokeAuraAction("EntryProbe", "run", nil)
			} else {
				invocation, err = machine.InvokeLWCMethod("EntryProbe", "run", nil)
			}
			if err != nil || !invocation.Success {
				t.Fatalf("invocation = %#v error = %#v, err = %v", invocation, invocation.Error, err)
			}
			if got := fmt.Sprint(invocation.ReturnValue); got != "1" {
				t.Fatalf("%s entry visible rows = %s, want 1", framework, got)
			}
		})
	}
}

func TestAsyncBodiesUseOwnAPIVersionForDatabaseDefaults(t *testing.T) {
	bodySource := "List<Account> rows = [SELECT Id, Secret__c FROM Account];"
	for _, test := range []struct {
		name   string
		caller string
		setup  func(string, ir.Program) Class
	}{
		{
			name:   "future",
			caller: "Test.startTest(); AsyncVersionWorker.run(); Test.stopTest();",
			setup: func(api string, body ir.Program) Class {
				return Class{
					Name:       "AsyncVersionWorker",
					APIVersion: api,
					Methods: map[string]Method{
						"run": {Name: "AsyncVersionWorker.run", ClassName: "AsyncVersionWorker", IsStatic: true, Modifiers: []string{"future"}, APIVersion: api, Program: body},
					},
				}
			},
		},
		{
			name:   "queueable",
			caller: "Test.startTest(); System.enqueueJob(new AsyncVersionWorker()); Test.stopTest();",
			setup: func(api string, body ir.Program) Class {
				return Class{
					Name:       "AsyncVersionWorker",
					APIVersion: api,
					Interfaces: []string{"Queueable"},
					Methods: map[string]Method{
						"execute": {Name: "AsyncVersionWorker.execute", ClassName: "AsyncVersionWorker", Params: []Param{{Name: "context", Type: "QueueableContext"}}, APIVersion: api, Program: body},
					},
				}
			},
		},
		{
			name:   "batch",
			caller: "Test.startTest(); Database.executeBatch(new AsyncVersionWorker(), 1); Test.stopTest();",
			setup: func(api string, body ir.Program) Class {
				start, _ := CompileAnonymousWithOptions("return new List<Account>{new Account(Name = 'scope')};", CompileOptions{APIVersion: api})
				finish, _ := CompileAnonymousWithOptions("return null;", CompileOptions{APIVersion: api})
				return Class{
					Name:       "AsyncVersionWorker",
					APIVersion: api,
					Interfaces: []string{"Database.Batchable<Account>"},
					Methods: map[string]Method{
						"start":   {Name: "AsyncVersionWorker.start", ClassName: "AsyncVersionWorker", ReturnType: "Iterable<Account>", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, APIVersion: api, Program: start},
						"execute": {Name: "AsyncVersionWorker.execute", ClassName: "AsyncVersionWorker", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}, {Name: "scope", Type: "List<Account>"}}, APIVersion: api, Program: body},
						"finish":  {Name: "AsyncVersionWorker.finish", ClassName: "AsyncVersionWorker", Params: []Param{{Name: "context", Type: "Database.BatchableContext"}}, APIVersion: api, Program: finish},
					},
				}
			},
		},
		{
			name:   "scheduled",
			caller: "Test.startTest(); System.schedule('nightly', '0 0 0 * * ?', new AsyncVersionWorker()); Test.stopTest();",
			setup: func(api string, body ir.Program) Class {
				return Class{
					Name:       "AsyncVersionWorker",
					APIVersion: api,
					Interfaces: []string{"Schedulable"},
					Methods: map[string]Method{
						"execute": {Name: "AsyncVersionWorker.execute", ClassName: "AsyncVersionWorker", Params: []Param{{Name: "context", Type: "SchedulableContext"}}, APIVersion: api, Program: body},
					},
				}
			},
		},
	} {
		for _, api := range []string{"66.0", "67.0"} {
			for _, callerAPI := range []string{"66.0", "67.0"} {
				t.Run(test.name+"/body"+api+"/caller"+callerAPI, func(t *testing.T) {
					body, err := CompileAnonymousWithOptions(bodySource, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					caller, err := CompileAnonymousWithOptions(test.caller, CompileOptions{APIVersion: callerAPI})
					if err != nil {
						t.Fatal(err)
					}
					machine := New(io.Discard)
					org := orgForSecurePolicyTest()
					machine.SetOrg(&org)
					machine.executionUser = stripInaccessibleTestUser()
					machine.EnableTestContext()
					if err := machine.RegisterClass(test.setup(api, body)); err != nil {
						t.Fatal(err)
					}
					_, err = machine.Execute(caller)
					if api == "67.0" {
						if err == nil {
							t.Fatal("API 67 async body unexpectedly bypassed field permissions")
						}
					} else if err != nil {
						t.Fatalf("API 66 async body unexpectedly used user-mode default: %v", err)
					}
				})
			}
		}
	}
}

func privateExecutionPolicyOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Widget__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:      "Widget__c",
			KeyPrefix:    "a00",
			SharingModel: "Private",
			Fields: map[string]storage.Field{
				"Id":   {APIName: "Id", Type: storage.FieldID},
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{
			"a00000000000001": {ID: "a00000000000001", Object: "Widget__c", System: storage.SystemFields{OwnerID: "005000000000001"}, Fields: map[string]storage.Value{"Name": storage.StringValue("Owned")}},
			"a00000000000002": {ID: "a00000000000002", Object: "Widget__c", System: storage.SystemFields{OwnerID: "005000000000002"}, Fields: map[string]storage.Value{"Name": storage.StringValue("Other")}},
		},
	}
	return org
}

func orgForSecurePolicyTest() storage.OrgState {
	return stripInaccessibleTestOrg()
}
