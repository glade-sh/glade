package apextest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestNetworkQueryResultBoundaries(t *testing.T) {
	// List/DTO operations and existing records are local data operations. Only
	// requiring a missing configuration record crosses the hosted boundary.
	unsupported := map[string]bool{
		"literalUnmatched":     true,
		"dynamicUnmatched":     true,
		"indexedBindUnmatched": true,
		"inlineFieldUnmatched": true,
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/CommunityQueryTest.cls"), `@IsTest private class CommunityQueryTest {
 public class CommunityView { public String Id; public String Name; }
 @IsTest static void literalSeeded() {
  Network record = [SELECT Id, Name FROM Network WHERE Name = 'Local Community'];
  System.assertEquals('Local Community',record.Name);
 }
 @IsTest static void literalUnmatched() {
  Network record = [SELECT Id, Name FROM Network WHERE Name = 'Missing Synthetic Community'];
 }
 @IsTest static void dynamicSeeded() {
  SObject record = Database.query('SELECT Id, Name FROM Network WHERE Name = \'Local Community\'');
  System.assertEquals('Local Community',record.get('Name'));
 }
 @IsTest static void dynamicUnmatched() {
  try {
   SObject record = Database.query('SELECT Id, Name FROM Network WHERE Name = \'Missing Synthetic Community\'');
  } catch (Exception caught) {
  }
 }
 @IsTest static void indexedBindUnmatched() {
  List<Id> keys = new List<Id>{UserInfo.getUserId()};
  Network record = [SELECT Id FROM Network WHERE Id = :keys[0]];
 }
 @IsTest static void inlineFieldUnmatched() {
  String name = [SELECT Name FROM Network WHERE Name = 'Missing Synthetic Community'].Name;
 }
 @IsTest static void countSeeded() {
  System.assertEquals(1,[SELECT COUNT() FROM Network WHERE Name = 'Local Community']);
 }
 @IsTest static void countUnmatched() {
  System.assertEquals(0,[SELECT COUNT() FROM Network WHERE Name = 'Missing Synthetic Community']);
 }
 @IsTest static void emptyList() {
  List<Network> records = [SELECT Id FROM Network WHERE Name = 'Missing Synthetic Community'];
  System.assertEquals(true,records.isEmpty());
 }
 @IsTest static void safeMissingField() {
  System.assertEquals(null,[SELECT Name FROM Network WHERE Name = 'Missing Synthetic Community']?.Name);
 }
 @IsTest static void mockReplacement() {
  Integer queries = Limits.getQueries();
  Set<Id> keys = new Set<Id>{UserInfo.getUserId()};
  List<SObject> records = Database.query('SELECT Id, Name FROM Network WHERE Id IN :keys');
  System.assertEquals(0,records.size());
  List<CommunityView> views = (List<CommunityView>)JSON.deserialize(JSON.serialize(records),List<CommunityView>.class);
  CommunityView mock = new CommunityView();
  mock.Id = UserInfo.getUserId();
  mock.Name = 'Synthetic community';
  views = new List<CommunityView>{mock};
  System.assertEquals(queries+1,Limits.getQueries());
  System.assertEquals('Synthetic community',views[0].Name);
 }
 @IsTest static void queriedProxyRoundTrip() {
  List<SObject> records = Database.query('SELECT Id, Name FROM Network LIMIT 3');
  List<CommunityView> views = (List<CommunityView>)JSON.deserialize(JSON.serialize(records),List<CommunityView>.class);
  System.assertEquals(records.size(),views.size());
  for(Integer i=0;i<records.size();i++) {
   System.assertEquals(String.valueOf(records[i].Id),views[i].Id);
   System.assertEquals(records[i].get('Name'),views[i].Name);
  }
 }
 @IsTest static void ordinaryEmptyQuery() {
  Boolean caught = false;
  try { Account record = [SELECT Id FROM Account WHERE Id = null]; }
  catch(QueryException failure) { caught = true; }
  System.assertEquals(true,caught);
 }
}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/CommunityQueryTest.cls-meta.xml"), fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api))
			index := loadTestIndex(t, root)
			org := orgFromIndex(index)
			if len(org.Objects["Network"].Records) == 0 {
				t.Fatal("expected deterministic Network metadata seed")
			}
			run := Run(index, Options{NoDiskCache: true})
			if run.Summary().Total != 13 {
				t.Fatalf("expected thirteen query cases: %#v", run)
			}
			for _, suite := range run.Suites {
				for _, row := range suite.Cases {
					want := testreport.StatusPass
					if unsupported[row.MethodName] {
						want = testreport.StatusUnsupported
					}
					if row.Status != want {
						t.Errorf("%s: status=%s want=%s reason=%s problem=%#v", row.MethodName, row.Status, want, row.Reason, row.Problem)
						continue
					}
					if want == testreport.StatusUnsupported && (row.Reason != testreport.ReasonUnsupportedFeature || row.Problem == nil || row.Problem.Type != "UnsupportedFeature" || !strings.Contains(row.Problem.Message, "Network configuration")) {
						t.Errorf("%s: expected explicit Network configuration boundary, got %#v", row.MethodName, row.Problem)
					}
				}
			}
		})
	}
}
