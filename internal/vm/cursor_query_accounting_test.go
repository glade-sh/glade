package vm

import (
	"errors"
	"fmt"
	"testing"
)

// Native captures retained four Salesforce observations at source APIs 62/67,
// sync/Queueable. The intervening versions below are local regressions only.
func TestApexCursorQueryAccountingVersioned(t *testing.T) {
	for api := 62; api <= 67; api++ {
		for _, profile := range []string{"strict-sync", "strict-async"} {
			t.Run(fmt.Sprintf("%d/%s", api, profile), func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
insert new Account(Name = 'Cursor accounting regression');
Integer queries = Limits.getQueries();
Integer rows = Limits.getQueryRows();
Integer cursorRows = Limits.getApexCursorRows();
Integer fetches = Limits.getFetchCallsOnApexCursor();
Database.Cursor cursor = Database.getCursor('SELECT Id FROM Account WHERE Name = ''Cursor accounting regression''');
System.assertEquals(1, cursor.getNumRecords());
System.assertEquals(queries, Limits.getQueries(), 'creation queries');
System.assertEquals(rows, Limits.getQueryRows(), 'creation query rows');
System.assertEquals(cursorRows + 1, Limits.getApexCursorRows(), 'creation cursor rows');
System.assertEquals(fetches, Limits.getFetchCallsOnApexCursor(), 'creation fetch calls');
for (Integer i = 1; i <= 2; i++) {
    List<SObject> page = cursor.fetch(0, 1);
    System.assertEquals(1, page.size());
    System.assertEquals(queries + i, Limits.getQueries(), 'fetch queries');
    System.assertEquals(rows + i, Limits.getQueryRows(), 'fetch query rows');
    System.assertEquals(cursorRows + 1, Limits.getApexCursorRows(), 'fetch cursor rows');
    System.assertEquals(fetches + i, Limits.getFetchCallsOnApexCursor(), 'fetch calls');
}
`, CompileOptions{APIVersion: fmt.Sprintf("%d.0", api)})
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := testDataOrg()
				machine.SetOrg(&org)
				machine.SetLimitMode(LimitModeStrict)
				caps, _ := LimitCapsForProfile(profile)
				machine.SetLimitCaps(caps)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

// Synthetic counters exercise local cap enforcement without allocating millions
// of records. These tests are not Salesforce maximum-row observations.
func TestApexCursorRowLimitStrictAndPermissive(t *testing.T) {
	for _, mode := range []LimitMode{LimitModeStrict, LimitModePermissive} {
		t.Run(string(mode), func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(mode)
			machine.limits.ApexCursorRows = 49_999_999
			if err := machine.incrementLimit("apexCursorRows", 1); err != nil {
				t.Fatal(err)
			}
			if len(machine.limitViolations) != 0 {
				t.Fatal(machine.limitViolations)
			}
			err := machine.incrementLimit("apexCursorRows", 1)
			if mode == LimitModeStrict {
				var runtimeErr *RuntimeError
				if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.LimitException" {
					t.Fatalf("overflow: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if len(machine.limitViolations) != 1 || machine.limitViolations[0] != (LimitViolation{Name: "apexCursorRows", Used: 50_000_001, Limit: 50_000_000}) {
				t.Fatal(machine.limitViolations)
			}
			machine.ResetLimits()
			if machine.limits.ApexCursorRows != 0 || len(machine.limitViolations) != 0 {
				t.Fatal("reset retained cursor rows")
			}
		})
	}
}

func TestApexCursorCreationDoesNotPreemptQueryBudget(t *testing.T) {
	for _, creation := range []string{
		`Database.getCursor('SELECT Id FROM Account WHERE Name = :name')`,
		`Database.getCursorWithBinds('SELECT Id FROM Account WHERE Name = :name', new Map<String,Object>{'name' => name}, null)`,
	} {
		t.Run(creation, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
String name = 'Cursor exhausted query budget';
insert new Account(Name = name);
Database.Cursor cursor = `+creation+`;
System.assertEquals(1, cursor.getNumRecords());
System.assertEquals(0, Limits.getQueries());
System.assertEquals(0, Limits.getQueryRows());
System.assertEquals(1, Limits.getApexCursorRows());
`, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			machine.SetLimitMode(LimitModeStrict)
			caps := defaultLimitCaps()
			caps.Queries, caps.QueryRows = 0, 0
			machine.SetLimitCaps(caps)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			_, err = machine.executeSOQL("SELECT Id FROM Account", &Result{})
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.LimitException" {
				t.Fatalf("ordinary query must still enforce budget: %v", err)
			}
		})
	}
}

func TestApexCursorPaginationAccountingUnchanged(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
insert new Account(Name = 'Pagination budget control');
Database.PaginationCursor cursor = Database.getPaginationCursor('SELECT Id FROM Account WHERE Name = ''Pagination budget control''');
System.assertEquals(1, Limits.getQueries());
System.assertEquals(1, Limits.getQueryRows());
System.assertEquals(0, Limits.getApexCursorRows());
cursor.fetchPage(0, 1);
System.assertEquals(1, Limits.getQueries());
System.assertEquals(1, Limits.getQueryRows());
System.assertEquals(1, Limits.getApexPaginationCursorRows());
System.assertEquals(0, Limits.getFetchCallsOnApexCursor());
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestApexCursorRowLimitVersionedGetter(t *testing.T) {
	for api := 62; api <= 67; api++ {
		t.Run(fmt.Sprint(api), func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`System.assertEquals(50000000, Limits.getLimitApexCursorRows());`, CompileOptions{APIVersion: fmt.Sprintf("%d.0", api)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := New(nil).Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestApexCursorFetchQueryAccounting(t *testing.T) {
	machine := New(nil)
	cursor := Object("Database.Cursor")
	cursor.Fields["Records"] = List(Object("Account"))
	machine.limits.ApexCursorRows = 1 // Already charged at creation.
	for i := 1; i <= 2; i++ {
		page, _, _, handled, err := machine.databaseCursorFetch(cursor, "fetch", []Value{Int(0), Int(1)}, false)
		if err != nil || !handled || len(page.List) != 1 {
			t.Fatalf("fetch %d: %v", i, err)
		}
		if machine.limits.Queries != i || machine.limits.QueryRows != i || machine.limits.ApexCursorRows != 1 || machine.limits.FetchCallsOnApexCursor != i {
			t.Fatalf("fetch %d accounting: %+v", i, machine.limits)
		}
	}
}

func TestApexCursorFetchEnforcesQueryBudgets(t *testing.T) {
	for _, mode := range []LimitMode{LimitModeStrict, LimitModePermissive} {
		for _, budget := range []string{"queries", "queryRows"} {
			t.Run(string(mode)+"/"+budget, func(t *testing.T) {
				machine := New(nil)
				machine.SetLimitMode(mode)
				caps := defaultLimitCaps()
				if budget == "queries" {
					caps.Queries = 0
				} else {
					caps.QueryRows = 0
				}
				machine.SetLimitCaps(caps)
				cursor := Object("Database.Cursor")
				cursor.Fields["Records"] = List(Object("Account"))
				machine.limits.ApexCursorRows = 1
				page, _, _, handled, err := machine.databaseCursorFetch(cursor, "fetch", []Value{Int(0), Int(1)}, false)
				if !handled {
					t.Fatal("fetch unhandled")
				}
				if mode == LimitModeStrict {
					var runtimeErr *RuntimeError
					if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.LimitException" || page.Kind != ValueNull {
						t.Fatalf("strict budget: %v, %+v", err, page)
					}
				} else if err != nil || len(page.List) != 1 {
					t.Fatalf("permissive budget: %v, %+v", err, page)
				}
				if len(machine.limitViolations) != 1 || machine.limitViolations[0] != (LimitViolation{Name: budget, Used: 1, Limit: 0}) {
					t.Fatal(machine.limitViolations)
				}
				if machine.limits.ApexCursorRows != 1 || machine.limits.FetchCallsOnApexCursor != 1 {
					t.Fatalf("fetch counters: %+v", machine.limits)
				}
			})
		}
	}
}
