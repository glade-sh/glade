package vm

import "testing"

// The owned dml_clock_native_observations fixture records that completed DML
// does not lead Datetime.now (source API61, anonymous and named routes). These
// independent consumers retain all sequential inserts in a through-now window.
func TestAuditWindowQueriesRetainCompletedInserts(t *testing.T) {
	const seed = `
Account first = new Account(Name='North', Phone='300'); insert first;
Account second = new Account(Name='North', Phone='200'); insert second;
Account third = new Account(Name='South', Phone='100'); insert third;
Datetime windowStart = Datetime.now().addDays(-1);
Datetime windowEnd = Datetime.now();
`
	for _, tc := range []struct{ name, body string }{
		{"count", `
System.assertEquals(3, [SELECT COUNT() FROM Account WHERE CreatedDate >= :windowStart AND CreatedDate <= :windowEnd]);
`},
		{"group and having", `
List<AggregateResult> groups = [SELECT Name, COUNT(Id) total FROM Account
    WHERE CreatedDate >= :windowStart AND CreatedDate <= :windowEnd
    GROUP BY Name HAVING COUNT(Id) >= 2];
System.assertEquals(1, groups.size());
System.assertEquals('North', groups[0].get('Name'));
System.assertEquals(2, groups[0].get('total'));
`},
		{"sort after filtering", `
List<Account> rows = [SELECT Id, Phone FROM Account
    WHERE CreatedDate >= :windowStart AND CreatedDate <= :windowEnd ORDER BY Phone ASC];
System.assertEquals(third.Id, rows[0].Id);
System.assertEquals(second.Id, rows[1].Id);
System.assertEquals(first.Id, rows[2].Id);
`},
		{"unbounded and earlier-window controls", `
System.assertEquals(3, [SELECT COUNT() FROM Account]);
List<Account> rows = [SELECT Id FROM Account ORDER BY Phone ASC];
System.assertEquals(third.Id, rows[0].Id);
System.assertEquals(second.Id, rows[1].Id);
System.assertEquals(first.Id, rows[2].Id);
System.assertEquals(0, [SELECT COUNT() FROM Account WHERE CreatedDate < :windowStart]);
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(seed+tc.body, CompileOptions{APIVersion: "61.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
