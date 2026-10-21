package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Owned SW01-SW03 named API62/67/68 observations retain a fresh fixed body
// candidate through a CreatedDate window, exclude an old one, and keep the
// default test search empty when fixed candidates were not provided.
func TestSOSLCreatedDateWindowPreservesSearchUnion(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, backdate string
			fixed          bool
			wantBody       int
		}{
			{"fresh fixed body", "", true, 1},
			{"old fixed body", "Test.setCreatedDate(body.Id, Datetime.now().addDays(-4));", true, 0},
			{"no fixed candidates control", "", false, 0},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				fixed, rawCount := "", 0
				if tc.fixed {
					fixed, rawCount = "Test.setFixedSearchResults(new List<Id>{body.Id});", 1
				}
				source := fmt.Sprintf(`
Account headline = new Account(Name='ownedneedle short');
Account body = new Account(Name='Owned body', Description='ownedneedle body text');
insert new List<Account>{headline,body};
%s
Datetime lower = Datetime.now().addDays(-1);
Datetime upper = Datetime.now();
String term = 'ownedneedle';
String pattern = '%%'+term+'%%';
%s
Test.startTest();
System.assertNotEquals(null, [SELECT CreatedDate FROM Account WHERE Id=:body.Id].CreatedDate);
System.assertEquals(%d, Search.query('FIND :term IN ALL FIELDS RETURNING Account(Id)', AccessLevel.SYSTEM_MODE)[0].size(), 'unfiltered fixed-result control');
String query = 'FIND :term IN ALL FIELDS RETURNING Account(Id WHERE CreatedDate >= '
    +lower.formatGmt('yyyy-MM-dd\'T\'HH:mm:ss\'Z\'')+' AND CreatedDate <= '
    +upper.formatGmt('yyyy-MM-dd\'T\'HH:mm:ss\'Z\'')+' ORDER BY CreatedDate DESC)';
Set<Id> bodyIds = new Set<Id>();
for (SObject row : Search.query(query, AccessLevel.SYSTEM_MODE)[0]) bodyIds.add(row.Id);
System.assertEquals(%d, bodyIds.size(), 'windowed body candidate');
Set<Id> headlineIds = new Map<Id,Account>([SELECT Id FROM Account
    WHERE Name LIKE :pattern AND CreatedDate >= :lower AND CreatedDate <= :upper]).keySet();
System.assertEquals(1, headlineIds.size(), 'short-field leg');
Set<Id> combined = new Set<Id>(); combined.addAll(headlineIds); combined.addAll(bodyIds);
System.assertEquals(%d, combined.size(), 'short/body union');
Test.stopTest();
`, tc.backdate, fixed, rawCount, tc.wantBody, 1+tc.wantBody)
				program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				org.APIVersion = api
				storage.EnsureStandardObject(&org, "Account")
				machine := New(nil)
				machine.SetOrg(&org)
				machine.EnableTestContext()
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
