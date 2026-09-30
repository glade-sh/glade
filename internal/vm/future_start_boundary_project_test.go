package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// Owned API 53 source accepted by the three-method Salesforce future boundary proof.
// Embedded source preserves class and project versions without plugin dependencies.
func TestFutureStartBoundaryAPI53Project(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"sourceApiVersion": "53.0", "packageDirectories": [{"path": "force-app", "default": true}]}`,
		"force-app/main/default/classes/GladeFutureBoundary134.cls": `public class GladeFutureBoundary134 {
    @future public static void countOpportunities(Set<Id> accountIds) {
        List<Account> updates = new List<Account>();
        for (Account a : [SELECT Id,(SELECT Id FROM Opportunities) FROM Account WHERE Id IN :accountIds]) {
            updates.add(new Account(Id=a.Id,NumberOfEmployees=a.Opportunities.size()));
        }
        update updates;
    }
}
`,
		"force-app/main/default/classes/GladeFutureBoundary134.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>
`,
		"force-app/main/default/classes/GladeFutureBoundaryTest134.cls": `@IsTest private class GladeFutureBoundaryTest134 {
    private static List<Account> setupAccounts() {
        List<Account> accounts = new List<Account>{new Account(Name='First',NumberOfEmployees=0),new Account(Name='Second',NumberOfEmployees=0)};
        insert accounts;
        List<Opportunity> opportunities = new List<Opportunity>();
        for (Account a : accounts) {
            opportunities.add(new Opportunity(Name='One',AccountId=a.Id,StageName='Closed Won',CloseDate=Date.today()));
            opportunities.add(new Opportunity(Name='Two',AccountId=a.Id,StageName='Closed Won',CloseDate=Date.today()));
        }
        insert opportunities;
        System.assertEquals(4,[SELECT COUNT() FROM Opportunity]);
        return accounts;
    }
    private static void assertCounts(List<Account> accounts,Integer first,Integer second) {
        Map<Id,Account> saved = new Map<Id,Account>([SELECT Id,NumberOfEmployees FROM Account]);
        System.assertEquals(2,saved.size(),'Both parent records persist');
        System.assertEquals(first,saved.get(accounts[0].Id).NumberOfEmployees,'First persisted future result');
        System.assertEquals(second,saved.get(accounts[1].Id).NumberOfEmployees,'Second persisted future result');
    }
    @IsTest static void futureBeforeStart() {
        List<Account> accounts=setupAccounts();
        GladeFutureBoundary134.countOpportunities(new Set<Id>{accounts[0].Id,accounts[1].Id});
        assertCounts(accounts,0,0);
        Test.startTest();
        Test.stopTest();
        assertCounts(accounts,2,2);
    }
    @IsTest static void futureInsideStart() {
        List<Account> accounts=setupAccounts();
        Test.startTest();
        GladeFutureBoundary134.countOpportunities(new Set<Id>{accounts[0].Id,accounts[1].Id});
        assertCounts(accounts,0,0);
        Test.stopTest();
        assertCounts(accounts,2,2);
    }
    @IsTest static void futuresAcrossStart() {
        List<Account> accounts=setupAccounts();
        GladeFutureBoundary134.countOpportunities(new Set<Id>{accounts[0].Id});
        Test.startTest();
        GladeFutureBoundary134.countOpportunities(new Set<Id>{accounts[1].Id});
        assertCounts(accounts,0,0);
        Test.stopTest();
        assertCounts(accounts,2,2);
    }
}
`,
		"force-app/main/default/classes/GladeFutureBoundaryTest134.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>
`,
	}
	for name, source := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, schema.Schema{})
	run := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: 1})
	count := 0
	for _, suite := range run.Suites {
		for _, tc := range suite.Cases {
			count++
			t.Run(tc.MethodName, func(t *testing.T) {
				if tc.Status != testreport.StatusPass {
					t.Fatalf("%s: %+v", tc.Status, tc.Problem)
				}
			})
		}
	}
	if count != 3 {
		t.Fatalf("executed %d methods; want all three", count)
	}
}
