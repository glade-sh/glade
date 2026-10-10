package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

func TestExecSOQLRejectsAggregateOnGroupedFieldAPI67(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
insert new Account(Name = 'Alpha', Industry = 'Technology', Rating = 'Hot');
insert new Account(Name = 'Beta', Industry = 'Technology', Rating = 'Hot');

Boolean rejectedSameField = false;
try {
    Database.query('SELECT Id, COUNT(Id) total FROM Account GROUP BY Id');
} catch (QueryException e) {
    rejectedSameField = e.getMessage().contains('Grouped field should not be aggregated: Id');
}
System.assert(rejectedSameField, 'a grouped field must not also be aggregated');

List<AggregateResult> byIndustry = [SELECT Industry, COUNT(Name) total FROM Account GROUP BY Industry];
System.assertEquals(1, byIndustry.size());
System.assertEquals('Technology', byIndustry[0].get('Industry'));
System.assertEquals(2, byIndustry[0].get('total'));

List<AggregateResult> byRating = [SELECT Rating, COUNT(Id) total FROM Account GROUP BY Rating];
System.assertEquals(1, byRating.size());
System.assertEquals('Hot', byRating[0].get('Rating'));
System.assertEquals(2, byRating[0].get('total'));
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}

	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Industry"] = storage.Field{
		APIName: "Industry", Type: storage.FieldPicklist, Groupable: storage.BoolFlag(true),
		PicklistValues: []storage.PicklistValue{{Value: "Technology", Label: "Technology", Active: true}},
	}
	account.Definition.Fields["Rating"] = storage.Field{
		APIName: "Rating", Type: storage.FieldString, Groupable: storage.BoolFlag(true),
	}
	org.Objects["Account"] = account
	if _, err := soql.ExecuteWithCache(org, soql.Query{
		Object: "Account", Fields: []string{"Id", "COUNT(Id) total"}, GroupBy: []string{"Id"},
	}, nil); err == nil || !strings.Contains(err.Error(), "Grouped field should not be aggregated: Id") {
		t.Fatalf("constructed grouped aggregate error = %v", err)
	}
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
