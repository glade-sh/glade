package vm

import (
	"strings"
	"testing"
)

// The exact observed API 62/67/68 cells are asserted by
// TestDynamicSOQLClosedScopeNativeConformance. These focused checks also retain
// uncaptured boundaries; only departed if-block String locals are admitted by
// the fallback.
func TestExecDynamicSOQLClosedIfStringCandidate(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name='Bound');
String query;
if(query==null){String wanted='Other';wanted='Bound';query='SELECT Id FROM Account WHERE Name=:wanted';}
System.assertEquals(1,Database.query(query).size());
System.assertEquals(1,Database.query('SELECT Id FROM Account WHERE Name=:WANTED',AccessLevel.SYSTEM_MODE).size());
System.assertEquals(1,Database.countQuery('SELECT count() FROM Account WHERE Name=:wanted'));
Database.QueryLocator locator=Database.getQueryLocator(query);
System.assert(locator.iterator().hasNext());
if(true){String missing=null;}
System.assertEquals(0,Database.query('SELECT Id FROM Account WHERE Name=:missing').size());
if(true){String wanted='Other';}
System.assertEquals(0,Database.query(query).size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.lookup("wanted"); err == nil {
		t.Fatal("departed local leaked into lexical lookup")
	}
}

func TestExecDynamicSOQLClosedLocalCandidateIsolation(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "BindFixture"}); err != nil {
		t.Fatal(err)
	}
	for _, method := range []struct{ name, source string }{
		{"query", "Database.query('SELECT Id FROM Account WHERE Name=:wanted');"},
		{"depart", "if(true){String wanted='Other';}"},
	} {
		program, err := CompileAnonymous(method.source)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterMethod(Method{Name: "BindFixture." + method.name, ClassName: "BindFixture", IsStatic: true, Program: program}); err != nil {
			t.Fatal(err)
		}
	}
	triggerProgram, err := CompileAnonymous(`
Boolean caught=false;
try{Database.query('SELECT Id FROM Account WHERE Name=:wanted');}catch(QueryException expected){caught=true;}
System.assert(caught);
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterTrigger(Trigger{Name: "BindScopeTrigger", Object: "Account", Operation: "insert", Timing: triggerTimingBefore, Program: triggerProgram}); err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
if(true){String wanted='Bound';}
insert new Account(Name='Bound');
Boolean caught=false;
try{BindFixture.query();}catch(QueryException expected){caught=true;}
System.assert(caught);
BindFixture.depart();
System.assertEquals(1,Database.query('SELECT Id FROM Account WHERE Name=:wanted').size());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	// Neither another top-level execution nor a clone inherits departed locals.
	for _, runtime := range []*VM{machine, machine.CloneRuntime(nil)} {
		query, err := CompileAnonymous("Database.query('SELECT Id FROM Account WHERE Name=:wanted');")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runtime.Execute(query); err == nil || !strings.Contains(err.Error(), "Variable does not exist: wanted") {
			t.Fatalf("frame leak: %v", err)
		}
	}
}

func TestExecDynamicSOQLClosedLocalCandidateBoundaries(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Account(Name='Bound');
String wanted='Bound';
if(true){String wanted='Other';}
System.assertEquals(1,Database.query('SELECT Id FROM Account WHERE Name=:wanted').size());
if(true){String departed='Bound';}
Boolean mapCaught=false;
try{Database.queryWithBinds('SELECT Id FROM Account WHERE Name=:departed',new Map<String,Object>(),AccessLevel.USER_MODE);}catch(QueryException expected){mapCaught=true;}
System.assert(mapCaught);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	// The candidate does not extend the unprobed type, bare-block or
	// inline-SOQL boundaries.
	for _, source := range []string{
		"if(true){Integer wanted=17;}Database.query('SELECT Id FROM Account WHERE NumberOfEmployees=:wanted');",
		"{String wanted='Bound';}Database.query('SELECT Id FROM Account WHERE Name=:wanted');",
		"if(true){String wanted='Bound';}List<Account> rows=[SELECT Id FROM Account WHERE Name=:wanted];",
	} {
		runtime := New(nil)
		runtime.SetOrg(&org)
		program, err := CompileAnonymous(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runtime.Execute(program); err == nil || !strings.Contains(err.Error(), "Variable does not exist: wanted") {
			t.Fatalf("unexpected boundary for %s: %v", source, err)
		}
	}
}

func TestExecDynamicSOQLCountHavingClosedLocalCandidate(t *testing.T) {
	program, err := CompileAnonymous(`
if(true){String wanted='Bound';}
String failureMessage;
try{Database.countQuery('SELECT COUNT(Id) FROM Account GROUP BY Name HAVING Name=:wanted');}
catch(QueryException failure){failureMessage=failure.getMessage();}
System.assertEquals('Use query() for non-count queries',failureMessage);
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(nil).Execute(program); err != nil {
		t.Fatal(err)
	}
}
