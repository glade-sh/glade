package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// The unchanged API53 two-method Salesforce Wave85 project.
func TestRunAggregateExpressionOrderContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeAggregateOrder53.cls"), "@IsTest private class GladeAggregateOrder53 {\n private static List<Contact> setup(){\n  List<Account> parents=new List<Account>{new Account(Name='Campaign Allocation'),new Account(Name='General')};insert parents;\n  List<Contact> rows=new List<Contact>{new Contact(LastName='opp0',AccountId=parents[0].Id),new Contact(LastName='opp1',AccountId=parents[0].Id),new Contact(LastName='opp2',AccountId=parents[1].Id),new Contact(LastName='opp3',AccountId=parents[1].Id),new Contact(LastName='opp4',AccountId=parents[1].Id),new Contact(LastName='outside',AccountId=parents[0].Id)};\n  insert rows;return rows;\n }\n @IsTest static void originalAggregateOrderAndTies(){\n  List<Contact> rows=setup();\n  List<AggregateResult> result=[SELECT count_distinct(Id), Account.Name FROM Contact WHERE LastName LIKE 'opp%' GROUP BY Account.Name ORDER BY count_distinct(Id)];\n  System.assertEquals(2,result.size(),'initial group population');\n  System.assertEquals(2,result[0].get('expr0'),'ascending smaller count');\n  System.assertEquals('Campaign Allocation',result[0].get('Name'),'ascending smaller group');\n  System.assertEquals(3,result[1].get('expr0'),'ascending larger count');\n  System.assertEquals('General',result[1].get('Name'),'ascending larger group');\n  delete rows[4];\n  result=[SELECT count_distinct(Id), Account.Name FROM Contact WHERE LastName LIKE 'opp%' GROUP BY Account.Name ORDER BY count_distinct(Id)];\n  System.assertEquals(2,result.size(),'equal-count group population');\n  Set<String> names=new Set<String>();for(AggregateResult row:result){System.assertEquals(2,row.get('expr0'),'equal counts without assumed tie order');names.add((String)row.get('Name'));}\n  System.assertEquals(new Set<String>{'Campaign Allocation','General'},names,'tie identities');\n }\n @IsTest static void spacedDynamicDescending(){\n  setup();\n  List<AggregateResult> result=Database.query('SELECT count_distinct ( Id ), Account.Name FROM Contact WHERE LastName LIKE \\'opp%\\' GROUP BY Account.Name ORDER BY count_distinct ( Id ) DESC');\n  System.assertEquals(2,result.size(),'dynamic group population');\n  System.assertEquals(3,result[0].get('expr0'),'descending larger count');\n  System.assertEquals('General',result[0].get('Name'),'descending larger group');\n  System.assertEquals(2,result[1].get('expr0'),'descending smaller count');\n  System.assertEquals('Campaign Allocation',result[1].get('Name'),'descending smaller group');\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeAggregateOrder53.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\n  \"sourceApiVersion\": \"53.0\",\n  \"packageDirectories\": [\n    {\n      \"path\": \"force-app\",\n      \"default\": true\n    }\n  ]\n}\n")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("exact aggregate order contracts failed: %s", data)
	}
}
