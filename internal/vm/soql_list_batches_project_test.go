package vm_test

import (
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"os"
	"path/filepath"
	"testing"
)

// SF191 admitted the namespaceempty API62/project40 contract. OwnedBatch is a
// local canonical-identity regression, not managed Salesforce namespace proof.
func TestSOQLListBatchesAPI62Project40(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, namespace := range []string{"", "OwnedBatch"} {
		t.Run("namespace="+namespace, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"force-app/main/default/classes/GladeListQuery191Proof.cls": `@IsTest private class GladeListQuery191Proof {
 @IsTest static void customQueryBatchesAndKeepsOrder() {
  List<GladeListQuery191__c> seed=new List<GladeListQuery191__c>();
  for(Integer i=0;i<201;i++) seed.add(new GladeListQuery191__c(Name='Owned '+i,Sequence__c=i));
  insert seed;
  Integer batches=0; Integer total=0;
  for(list<GladeListQuery191__c> listRows : [select Id,Sequence__c from GladeListQuery191__c order by Sequence__c]) {
   System.assertEquals(batches==0?200:1,listRows.size(),'custom chunk size');
   for(GladeListQuery191__c row:listRows) { System.assertEquals(Decimal.valueOf(String.valueOf(total)),row.Sequence__c,'custom order'); total++; }
   batches++;
  }
  System.assertEquals(2,batches); System.assertEquals(201,total);
  List<GladeListQuery191__c> loaded=[select Id from GladeListQuery191__c];
  Integer ordinaryRows=0; for(GladeListQuery191__c row:loaded) ordinaryRows++;
  System.assertEquals(201,ordinaryRows,'ordinary scalar list loop');
  Integer emptyBodies=0;
  for(list<GladeListQuery191__c> emptyRows : [select Id from GladeListQuery191__c where Sequence__c<0]) { emptyBodies++; System.assertEquals(0,emptyRows.size(),'empty custom batch size'); }
  System.assertEquals(1,emptyBodies,'empty custom query invokes one empty batch');
 }
 @IsTest static void standardQueryBatchesAndKeepsOrder() {
  List<Account> seed=new List<Account>();
  for(Integer i=0;i<201;i++) seed.add(new Account(Name='Owned '+i,AnnualRevenue=i));
  insert seed;
  Integer batches=0; Integer total=0;
  for(List<Account> listRows : [select Id,AnnualRevenue from Account order by AnnualRevenue]) {
   System.assertEquals(batches==0?200:1,listRows.size(),'standard chunk size');
   for(Account row:listRows) { System.assertEquals(Decimal.valueOf(String.valueOf(total)),row.AnnualRevenue,'standard order'); total++; }
   batches++;
  }
  System.assertEquals(2,batches); System.assertEquals(201,total);
  Integer emptyBodies=0;
  for(List<Account> emptyRows : [select Id from Account where AnnualRevenue<0]) { emptyBodies++; System.assertEquals(0,emptyRows.size(),'empty standard batch size'); }
  System.assertEquals(1,emptyBodies,'empty standard query invokes one empty batch');
 }
}
`,
				"force-app/main/default/classes/GladeListQuery191Proof.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`,
				"force-app/main/default/objects/GladeListQuery191__c/GladeListQuery191__c.object-meta.xml": `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>List Query Row</label><pluralLabel>List Query Rows</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>
`,
				"force-app/main/default/objects/GladeListQuery191__c/fields/Sequence__c.field-meta.xml": `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Sequence__c</fullName><label>Sequence</label><type>Number</type><precision>9</precision><scale>0</scale></CustomField>
`,
			}
			files["sfdx-project.json"] = `{"namespace":"` + namespace + `","sourceApiVersion":"40.0","packageDirectories":[{"path":"force-app","default":true}]}`
			for name, content := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			s, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			report := apextest.Run(typesys.Build(p, s), apextest.Options{NoDiskCache: true})
			if got := report.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
				for _, suite := range report.Suites {
					for _, c := range suite.Cases {
						if c.Problem != nil {
							t.Logf("%s: %s: %s", c.MethodName, c.Problem.Type, c.Problem.Message)
						}
					}
				}
				t.Fatalf("batch contract: %+v", got)
			}
		})
	}
}
