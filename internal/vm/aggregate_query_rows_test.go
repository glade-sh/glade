package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The Apex Developer Guide, Working with SOQL Aggregate Functions, specifies
// source-row charging for non-COUNT aggregates; COUNT charges result groups.
// https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_developer_guide.pdf
// Owned governor R086/R105/R108/R111 also record the one-row COUNT exception.
func TestExecAggregateSOQLQueryRowAccounting(t *testing.T) {
	cases := []struct {
		name, query string
		rows        int
	}{
		{"groupedSum", "SELECT Name, SUM(Amount__c) total FROM Account GROUP BY Name", 4},
		{"ungroupedSum", "SELECT SUM(Amount__c) total FROM Account", 4},
		{"filteredSum", "SELECT Name, SUM(Amount__c) total FROM Account WHERE Amount__c > 1 GROUP BY Name", 3},
		{"mixedSumAndCount", "SELECT Name, SUM(Amount__c) total, COUNT(Id) number FROM Account GROUP BY Name", 4},
		{"groupedAverage", "SELECT Name, AVG(Amount__c) total FROM Account GROUP BY Name", 4},
		{"groupedMinimum", "SELECT Name, MIN(Amount__c) total FROM Account GROUP BY Name", 4},
		{"groupedMaximum", "SELECT Name, MAX(Amount__c) total FROM Account GROUP BY Name", 4},
		{"groupedDistinctCount", "SELECT Name, COUNT_DISTINCT(Amount__c) total FROM Account GROUP BY Name", 4},
		{"scalarCountControl", "SELECT COUNT() FROM Account", 1},
		{"fieldCountControl", "SELECT COUNT(Id) total FROM Account", 1},
		{"groupedCountControl", "SELECT Name, COUNT(Id) total FROM Account GROUP BY Name", 2},
		{"emptyCountControl", "SELECT COUNT(Id) total FROM Account WHERE Id = null", 1},
		{"ordinaryControl", "SELECT Id FROM Account", 4},
	}
	for _, tc := range cases {
		for _, dynamic := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dynamic=%t", tc.name, dynamic), func(t *testing.T) {
				expression := "[" + tc.query + "]"
				if dynamic {
					method := "query"
					if tc.name == "scalarCountControl" {
						method = "countQuery"
					}
					expression = "Database." + method + "('" + strings.ReplaceAll(tc.query, "'", "\\'") + "')"
				}
				program, err := CompileAnonymous(fmt.Sprintf(`
Integer queriesBefore = Limits.getQueries();
Integer rowsBefore = Limits.getQueryRows();
Object selected = %s;
System.assertEquals(queriesBefore + 1, Limits.getQueries(), 'one query');
System.assertEquals(rowsBefore + %d, Limits.getQueryRows(), 'aggregate row accounting');
`, expression, tc.rows))
				if err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				org := aggregateQueryRowsOrg()
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestExecTriggerAggregateSOQLQueryRows(t *testing.T) {
	trigger, err := CompileAnonymous(`
Integer queriesBefore = Limits.getQueries();
Integer rowsBefore = Limits.getQueryRows();
List<AggregateResult> totals = [SELECT Name, SUM(Amount__c) total FROM Account GROUP BY Name];
System.assertEquals(2, totals.size(), 'two aggregate outputs');
System.assertEquals(queriesBefore + 1, Limits.getQueries());
System.assertEquals(rowsBefore + 4, Limits.getQueryRows(), 'four aggregate inputs');
`)
	if err != nil {
		t.Fatal(err)
	}
	program, err := CompileAnonymous(`
insert new Contact(LastName = 'ledger');
System.assertEquals(1, Limits.getQueries(), 'no double charging');
System.assertEquals(4, Limits.getQueryRows(), 'trigger shares source-row charge');
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := aggregateQueryRowsOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if err := machine.RegisterTrigger(Trigger{Name: "AggregateLedger", Object: "Contact", Timing: triggerTimingAfter, Operation: "insert", Program: trigger}); err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// These are local compatibility controls, not new Salesforce observations.
// Contributor accounting for these boundaries remains awaiting native rows.
func TestAggregateQueryRowsPreservesUncapturedBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name, query string
		rows        int
		nullInput   bool
	}{
		{"limit", "SELECT Name, SUM(Amount__c) total FROM Account GROUP BY Name LIMIT 1", 1, false},
		{"offset", "SELECT Name, SUM(Amount__c) total FROM Account GROUP BY Name LIMIT 1 OFFSET 1", 1, false},
		{"having", "SELECT Name, SUM(Amount__c) total FROM Account GROUP BY Name HAVING SUM(Amount__c) > 5", 1, false},
		{"hiddenHaving", "SELECT Name, COUNT(Id) total FROM Account GROUP BY Name HAVING SUM(Amount__c) > 5", 1, false},
		{"rollup", "SELECT Name, SUM(Amount__c) total FROM Account GROUP BY ROLLUP(Name)", 3, false},
		{"grouping", "SELECT Name, COUNT(Id) total, GROUPING(Name) subtotal FROM Account GROUP BY ROLLUP(Name)", 3, false},
		{"nullInput", "SELECT Name, SUM(Amount__c) total FROM Account GROUP BY Name", 2, true},
		{"emptyInput", "SELECT SUM(Amount__c) total FROM Account WHERE Id = null", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := aggregateQueryRowsOrg()
			if tc.nullInput {
				account := org.Objects["Account"]
				row := account.Records["001000000000100"]
				delete(row.Fields, "Amount__c")
				account.Records[row.ID] = row
				org.Objects["Account"] = account
			}
			program, err := CompileAnonymous(fmt.Sprintf(`
Object selected = Database.query('%s');
System.assertEquals(1, Limits.getQueries());
System.assertEquals(%d, Limits.getQueryRows(), 'existing local boundary');
`, tc.query, tc.rows))
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Sharing restricts contributors before aggregation. Three owned inputs form
// two groups; a fourth, other-owned input must affect neither output nor budget.
func TestAggregateQueryRowsPreservesSharingBoundary(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		t.Run(fmt.Sprintf("dynamic=%t", dynamic), func(t *testing.T) {
			query := "SELECT Name, SUM(Amount__c) total FROM Account GROUP BY Name"
			expression := "[" + query + "]"
			projection := "[SELECT Id FROM Account]"
			count := "[SELECT COUNT() FROM Account]"
			sum := "[SELECT SUM(Amount__c) total FROM Account]"
			if dynamic {
				expression = "Database.query('" + query + "')"
				projection = "Database.query('SELECT Id FROM Account')"
				count = "Database.countQuery('SELECT COUNT() FROM Account')"
				sum = "Database.query('SELECT SUM(Amount__c) total FROM Account')"
			}
			program, err := CompileAnonymousWithOptions(fmt.Sprintf(`
List<AggregateResult> totals = %s;
System.assertEquals(2, totals.size(), 'two visible aggregate groups');
System.assertEquals(1, Limits.getQueries(), 'one aggregate query');
System.assertEquals(3, Limits.getQueryRows(), 'three visible aggregate inputs');
Decimal sum = 0;
for (AggregateResult row : totals) sum += (Decimal)row.get('total');
System.assertEquals(9, sum, 'other owner input excluded');
List<Account> visible = %s;
System.assertEquals(3, visible.size(), 'projection visibility control');
System.assertEquals(6, Limits.getQueryRows(), 'projection adds three rows');
Integer counted = %s;
System.assertEquals(3, counted, 'COUNT visibility control');
System.assertEquals(7, Limits.getQueryRows(), 'COUNT adds one result row');
List<AggregateResult> scalar = %s;
System.assertEquals(9, scalar[0].get('total'), 'scalar SUM visibility control');
System.assertEquals(10, Limits.getQueryRows(), 'scalar SUM adds three inputs');
System.assertEquals(4, Limits.getQueries(), 'no duplicate query charges');
`, expression, projection, count, sum), CompileOptions{APIVersion: "66.0"})
			if err != nil {
				t.Fatal(err)
			}
			org := aggregateQueryRowsOrg()
			account := org.Objects["Account"]
			account.Definition.SharingModel = "Private"
			for id, record := range account.Records {
				record.System.OwnerID = "005000000000002"
				if id == "001000000000100" {
					record.System.OwnerID = "005000000000001"
				}
				account.Records[id] = record
			}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(storage.Record{ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue("005000000000002")}})
			if err := machine.RegisterClass(Class{Name: "LedgerReader", APIVersion: "66.0", Modifiers: []string{"with sharing"}}); err != nil {
				t.Fatal(err)
			}
			if _, err := machine.ExecuteInClass(program, "LedgerReader"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func aggregateQueryRowsOrg() storage.OrgState {
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Amount__c"] = storage.Field{APIName: "Amount__c", Type: storage.FieldDecimal}
	for i, name := range []string{"first", "first", "first", "second"} {
		id := storage.ID(fmt.Sprintf("0010000000001%02d", i))
		account.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{
			"Name": storage.StringValue(name), "Amount__c": storage.IntegerValue(int64(i + 1)),
		}}
	}
	org.Objects["Account"] = account
	return org
}
