package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecAggregateResultShapeContract(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
insert new Account(Name = 'A', AnnualRevenue = 10, Rating = 'Hot');
insert new Account(Name = 'B', AnnualRevenue = 20, Rating = 'Hot');
insert new Account(Name = 'C', AnnualRevenue = 30, Rating = 'Warm');

Integer inlineCountAll = [SELECT COUNT() FROM Account];
System.assertEquals(3, inlineCountAll);

// Bare expressions must preserve aggregate shape too; assignment targets
// take executeSOQLForType and can hide an inline scalarization regression.
System.assert([SELECT COUNT(Id) FROM Account] instanceof List<AggregateResult>, 'bare inline COUNT(Id) must retain AggregateResult list shape');
System.assert([SELECT AVG(AnnualRevenue) FROM Account] instanceof List<AggregateResult>, 'bare inline AVG must retain AggregateResult list shape');
System.assert([SELECT MAX(AnnualRevenue) FROM Account] instanceof List<AggregateResult>, 'bare inline MAX must retain AggregateResult list shape');

Object rawCountRows = [SELECT COUNT(Id) FROM Account];
System.assert(rawCountRows instanceof List<AggregateResult>);
List<AggregateResult> countFieldRows = (List<AggregateResult>)rawCountRows;
System.assertEquals(1, countFieldRows.size());
System.assertEquals(3, countFieldRows.get(0).get('expr0'));

Object rawAverageRows = [SELECT AVG(AnnualRevenue) FROM Account];
System.assert(rawAverageRows instanceof List<AggregateResult>);
List<AggregateResult> averageRows = (List<AggregateResult>)rawAverageRows;
System.assertEquals(1, averageRows.size());
System.assertEquals(20.0, averageRows.get(0).get('expr0'));
Object rawMaximumRows = [SELECT MAX(AnnualRevenue) FROM Account];
System.assert(rawMaximumRows instanceof List<AggregateResult>);
List<AggregateResult> maximumRows = (List<AggregateResult>)rawMaximumRows;
System.assertEquals(1, maximumRows.size());
System.assertEquals(30.0, maximumRows.get(0).get('expr0'));

List<AggregateResult> groupedRows = [SELECT Rating, COUNT(Id) groupCount FROM Account GROUP BY Rating ORDER BY Rating];
System.assertEquals(2, groupedRows.size());
System.assertEquals('Hot', groupedRows.get(0).get('Rating'));
System.assertEquals(2, groupedRows.get(0).get('groupCount'));
System.assertEquals('Warm', groupedRows.get(1).get('Rating'));
System.assertEquals(1, groupedRows.get(1).get('groupCount'));

List<SObject> dynamicCountRows = Database.query('SELECT COUNT(Id) dynamicCount FROM Account');
System.assertEquals(1, dynamicCountRows.size());
System.assertEquals(3, dynamicCountRows.get(0).get('dynamicCount'));
List<SObject> dynamicCountAllRows = Database.query('SELECT COUNT() FROM Account');
System.assertEquals(1, dynamicCountAllRows.size());
System.assertEquals(3, dynamicCountAllRows.get(0).get('expr0'));
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}

	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["AnnualRevenue"] = storage.Field{APIName: "AnnualRevenue", Type: storage.FieldDecimal}
	account.Definition.Fields["Rating"] = storage.Field{APIName: "Rating", Type: storage.FieldString}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
