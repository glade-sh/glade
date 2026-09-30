package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Original six API53 SOSL escape contracts passed on Salesforce in Wave72.
func TestRunSearchEscapeContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSearchEscapes53.cls"), "@IsTest private class GladeSearchEscapes53 {\n private static void check(String name,String searchTerm,Boolean wildcard,Boolean whereName){\n  Contact first=new Contact(LastName=name),second=new Contact(LastName=name);insert new List<Contact>{first,second};\n  Test.setFixedSearchResults(new List<Id>{first.Id,second.Id});\n  String term=String.escapeSingleQuotes(searchTerm)+(wildcard?'*':'');\n  String suffix=' RETURNING Contact(Id,LastName'+(whereName?' WHERE LastName = \\''+String.escapeSingleQuotes(name)+'\\'':'')+')';\n  String query=String.format('FIND \\'\\'{0}\\'\\' IN ALL FIELDS',new String[]{term})+suffix;\n  System.assertEquals('FIND \\''+term+'\\' IN ALL FIELDS'+suffix,query,'Formatted search must retain escapes');\n  Test.startTest();List<List<SObject>> groups=Search.query(query);Test.stopTest();\n  System.assertEquals(1,groups.size());System.assertEquals(2,groups[0].size());\n  Set<Id> ids=new Set<Id>();for(SObject row:groups[0]){ids.add((Id)row.get('Id'));System.assertEquals(name,row.get('LastName'));}\n  System.assertEquals(new Set<Id>{first.Id,second.Id},ids);\n }\n @IsTest static void plainControl(){check('Sullivan','Sullivan',false,false);}\n @IsTest static void apostropheWildcard(){check('O\\'Sullivan','O\\'Sullivan',true,false);}\n @IsTest static void apostropheExact(){check('O\\'Sullivan','O\\'Sullivan',false,false);}\n @IsTest static void multipleApostrophes(){check('D\\'Angelo O\\'Neil','D\\'Angelo O\\'Neil',true,false);}\n @IsTest static void returningEscapedString(){check('O\\'Sullivan','Sullivan',false,true);}\n @IsTest static void escapedBackslash(){check('TrailPath','Trail\\\\\\\\Path',false,false);}\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSearchEscapes53.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"53.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}\n")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 6 || got.Passed != 6 || got.Skipped != 0 || got.Failed != 0 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("search escape contracts failed: %s", data)
	}
}
