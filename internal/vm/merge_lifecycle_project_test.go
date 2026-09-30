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

// Owned API 53 source accepted by the four-method Salesforce merge lifecycle proof.
// Keep this project self-contained; it has no dependency on corpus/plugin files.
func TestMergeLifecycleAPI53Project(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"sourceApiVersion": "53.0", "packageDirectories": [{"path": "force-app", "default": true}]}`,
		"force-app/main/default/classes/GladeMergeProbe53.cls": `public class GladeMergeProbe53 {
    public static Boolean enabled = false;
    public static Id targetId;
    public static Integer seen = 0;
    public static Id newAccountId;
    public static Id oldAccountId;
    public static String newLastName;
    public static String oldLastName;
}
`,
		"force-app/main/default/classes/GladeMergeProbe53.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>
`,
		"force-app/main/default/classes/GladeMergeLifecycle53.cls": `@IsTest private class GladeMergeLifecycle53 {
    @IsTest static void sparseContactMasterHydrated() {
        List<Account> accounts = new List<Account>{new Account(Name='Winner account'),new Account(Name='Other account')};
        insert accounts;
        Contact winner = new Contact(LastName='Retained winner',AccountId=accounts[0].Id);
        Contact loser = new Contact(LastName='Other contact',AccountId=accounts[1].Id);
        insert new List<Contact>{winner,loser};
        GladeMergeProbe53.targetId = winner.Id;
        GladeMergeProbe53.enabled = true;
        try {
            Contact sparseWinner = new Contact(Id=winner.Id);
            merge sparseWinner loser;
        } finally {
            GladeMergeProbe53.enabled = false;
        }
        System.assertEquals(1,GladeMergeProbe53.seen,'Targeted merge must invoke before update once');
        System.assertEquals(accounts[0].Id,GladeMergeProbe53.newAccountId,'Trigger.new must contain retained AccountId');
        System.assertEquals(accounts[0].Id,GladeMergeProbe53.oldAccountId,'Trigger.old must contain prior AccountId');
        System.assertEquals('Retained winner',GladeMergeProbe53.newLastName,'Trigger.new must contain retained LastName');
        System.assertEquals('Retained winner',GladeMergeProbe53.oldLastName,'Trigger.old must contain prior LastName');
        Contact saved = [SELECT AccountId,LastName FROM Contact WHERE Id=:winner.Id];
        System.assertEquals(accounts[0].Id,saved.AccountId,'Sparse merge must retain winner Account');
        System.assertEquals('Retained winner',saved.LastName);
        System.assertEquals(0,[SELECT COUNT() FROM Contact WHERE Id=:loser.Id]);
    }

    private static void assertAccountChildrenReparent(Boolean shortChildReference, Boolean shortMergeInput) {
        Account winner = new Account(Name='Retained account');
        Account loser = new Account(Name='Merged account');
        insert new List<Account>{winner,loser};
        Id childParentId = loser.Id;
        if (shortChildReference) {
            String shortText = String.valueOf(loser.Id).substring(0,15);
            System.assertEquals(15,shortText.length());
            childParentId = (Id)shortText;
        }
        Contact child = new Contact(LastName='Retained child',AccountId=childParentId);
        insert child;
        Opportunity opportunity = new Opportunity(Name='Retained opportunity',AccountId=childParentId,StageName='Closed Won',CloseDate=Date.today(),Amount=500);
        insert opportunity;
        System.assertEquals(loser.Id,[SELECT AccountId FROM Contact WHERE Id=:child.Id].AccountId,'Contact baseline parent');
        System.assertEquals(loser.Id,[SELECT AccountId FROM Opportunity WHERE Id=:opportunity.Id].AccountId,'Opportunity baseline parent');
        Id winnerInput = winner.Id;
        Id loserInput = loser.Id;
        if (shortMergeInput) {
            winnerInput = (Id)String.valueOf(winner.Id).substring(0,15);
            loserInput = (Id)String.valueOf(loser.Id).substring(0,15);
        }
        Account master = new Account(Id=winnerInput);
        Account duplicate = new Account(Id=loserInput);
        merge master duplicate;
        System.assertEquals(1,[SELECT COUNT() FROM Account WHERE Id=:winner.Id]);
        System.assertEquals(0,[SELECT COUNT() FROM Account WHERE Id=:loser.Id]);
        Contact savedContact = [SELECT AccountId FROM Contact WHERE Id=:child.Id];
        Opportunity savedOpportunity = [SELECT AccountId,Amount FROM Opportunity WHERE Id=:opportunity.Id];
        System.assertEquals(winner.Id,savedContact.AccountId,'Contact child must survive and reparent');
        System.assertEquals(winner.Id,savedOpportunity.AccountId,'Opportunity child must survive and reparent');
        System.assertEquals(500,savedOpportunity.Amount);
        System.assertEquals(0,[SELECT COUNT() FROM Contact WHERE AccountId=:loser.Id]);
        System.assertEquals(0,[SELECT COUNT() FROM Opportunity WHERE AccountId=:loser.Id]);
    }
    @IsTest static void accountChildrenFullIds() { assertAccountChildrenReparent(false,false); }
    @IsTest static void accountChildrenShortReferences() { assertAccountChildrenReparent(true,false); }
    @IsTest static void accountChildrenShortMergeInputs() { assertAccountChildrenReparent(false,true); }
}
`,
		"force-app/main/default/classes/GladeMergeLifecycle53.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>
`,
		"force-app/main/default/triggers/GladeMergeContact130.trigger": `trigger GladeMergeContact130 on Contact (before update) {
    if (GladeMergeProbe53.enabled) {
        for (Contact current : Trigger.new) {
            if (current.Id == GladeMergeProbe53.targetId) {
                Contact prior = Trigger.oldMap.get(current.Id);
                GladeMergeProbe53.seen++;
                GladeMergeProbe53.newAccountId = current.AccountId;
                GladeMergeProbe53.oldAccountId = prior.AccountId;
                GladeMergeProbe53.newLastName = current.LastName;
                GladeMergeProbe53.oldLastName = prior.LastName;
            }
        }
    }
}
`,
		"force-app/main/default/triggers/GladeMergeContact130.trigger-meta.xml": `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>53.0</apiVersion><status>Active</status></ApexTrigger>
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
	if count != 4 {
		t.Fatalf("executed %d methods; want all four", count)
	}
}
