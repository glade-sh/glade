package vm

import (
	"errors"
	"fmt"
	"testing"
)

// TASK-6.1.25 owned Salesforce classes observed getter 100 at APIs 62–67.
// Queueable fetches 1–100 succeeded and fetch 101 hit the cursor limit.
// Sync reached 99 before SOQL preemption; these isolated cap tests do not
// establish sync upper-bound parity or cursor SOQL/row-budget parity.
func TestApexCursorFetchLimitVersionedGetter(t *testing.T) {
	for api := 62; api <= 67; api++ {
		t.Run(fmt.Sprint(api), func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
System.assertEquals(100, Limits.getLimitFetchCallsOnApexCursor());
System.assertEquals(0, Limits.getFetchCallsOnApexCursor());
insert new Account(Name = 'Cursor limit regression');
Database.Cursor cursor = Database.getCursor('SELECT Id FROM Account WHERE Name = ''Cursor limit regression''');
List<SObject> page = cursor.fetch(0, 1);
System.assertEquals(1, page.size());
System.assertEquals(1, Limits.getFetchCallsOnApexCursor());
`, CompileOptions{APIVersion: fmt.Sprintf("%d.0", api)})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			org := testDataOrg()
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestApexCursorFetchLimitStrictAndPermissive(t *testing.T) {
	for _, mode := range []LimitMode{LimitModeStrict, LimitModePermissive} {
		t.Run(string(mode), func(t *testing.T) {
			machine := New(nil)
			machine.SetLimitMode(mode)
			// Isolate the fetch cap under the observed Queueable query budget.
			// Sync query/query-row preemption has separate accounting controls.
			caps, _ := LimitCapsForProfile("strict-async")
			machine.SetLimitCaps(caps)
			cursor := Object("Database.Cursor")
			records := make([]Value, 101)
			for i := range records {
				records[i] = Object("Account")
			}
			cursor.Fields["Records"] = List(records...)
			for i := 0; i < 100; i++ {
				page, _, _, handled, err := machine.databaseCursorFetch(cursor, "fetch", []Value{Int(int64(i)), Int(1)}, false)
				if err != nil || !handled || len(page.List) != 1 {
					t.Fatalf("fetch %d: handled=%t records=%d error=%v", i+1, handled, len(page.List), err)
				}
				if machine.limits.FetchCallsOnApexCursor != i+1 {
					t.Fatalf("fetch %d counter=%d", i+1, machine.limits.FetchCallsOnApexCursor)
				}
			}
			if len(machine.limitViolations) != 0 {
				t.Fatalf("violations at the boundary: %+v", machine.limitViolations)
			}
			page, _, _, handled, err := machine.databaseCursorFetch(cursor, "fetch", []Value{Int(100), Int(1)}, false)
			if !handled {
				t.Fatal("cursor fetch was not handled")
			}
			if mode == LimitModeStrict {
				var runtimeErr *RuntimeError
				if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.LimitException" {
					t.Fatalf("fetch 101 error=%v, want System.LimitException", err)
				}
				if page.Kind != ValueNull {
					t.Fatalf("strict overflow returned records: %+v", page)
				}
			} else if err != nil || len(page.List) != 1 {
				t.Fatalf("permissive fetch 101: records=%d error=%v", len(page.List), err)
			}
			if machine.limits.FetchCallsOnApexCursor != 101 {
				t.Fatalf("overflow counter=%d, want 101", machine.limits.FetchCallsOnApexCursor)
			}
			if len(machine.limitViolations) != 1 || machine.limitViolations[0] != (LimitViolation{Name: "fetchCallsOnApexCursor", Used: 101, Limit: 100}) {
				t.Fatalf("overflow violation=%+v", machine.limitViolations)
			}
			machine.ResetLimits()
			if machine.limits.FetchCallsOnApexCursor != 0 || len(machine.limitViolations) != 0 {
				t.Fatal("reset retained cursor counters or violations")
			}
			if getter, ok := machine.limitValue("getLimitFetchCallsOnApexCursor"); !ok || getter.Int != 100 {
				t.Fatalf("getter after reset=%+v, handled=%t", getter, ok)
			}
			if _, _, _, _, err := machine.databaseCursorFetch(cursor, "fetch", []Value{Int(0), Int(1)}, false); err != nil {
				t.Fatalf("fetch after reset: %v", err)
			}
			if machine.limits.FetchCallsOnApexCursor != 1 {
				t.Fatal("fetch after reset did not start a new counter")
			}
		})
	}
}
