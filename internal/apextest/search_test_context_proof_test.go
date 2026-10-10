package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Original five API53 SOSL test-context contracts passed on Salesforce in Wave73.
func TestRunSearchTestContextContracts(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSearchState53.cls"), "@IsTest private class GladeSearchState53 {\n private static List<SObject> rows(String query){List<List<SObject>> groups=Search.query(query);System.assertEquals(1,groups.size(),'Returning group retained');return groups[0];}\n private static void expectIds(List<SObject> actual,Set<Id> expected){System.assertEquals(expected.size(),actual.size());Set<Id> ids=new Set<Id>();for(SObject row:actual)ids.add((Id)row.get('Id'));System.assertEquals(expected,ids);}\n @IsTest static void defaultDynamicEmptyAllPhases(){\n  insert new Contact(LastName='Owned Search Match');String query='FIND \\'Owned*\\' IN NAME FIELDS RETURNING Contact(Id,LastName)';\n  expectIds(rows(query),new Set<Id>());Test.startTest();expectIds(rows(query),new Set<Id>());Test.stopTest();expectIds(rows(query),new Set<Id>());\n }\n @IsTest static void defaultInlineEmpty(){\n  insert new Contact(LastName='Owned Search Match');List<List<SObject>> groups=[FIND 'Owned*' IN NAME FIELDS RETURNING Contact(Id,LastName)];System.assertEquals(1,groups.size());expectIds(groups[0],new Set<Id>());\n }\n @IsTest static void explicitEmptyResetsFixedResults(){\n  Contact row=new Contact(LastName='Owned Search Match');insert row;String query='FIND \\'Owned*\\' IN NAME FIELDS RETURNING Contact(Id)';\n  Test.setFixedSearchResults(new List<Id>{row.Id});expectIds(rows(query),new Set<Id>{row.Id});Test.setFixedSearchResults(new List<Id>());expectIds(rows(query),new Set<Id>());\n }\n @IsTest static void fixedReplacementAndStartStop(){\n  Contact first=new Contact(LastName='Owned One'),second=new Contact(LastName='Owned Two');insert new List<Contact>{first,second};String query='FIND \\'Owned*\\' IN NAME FIELDS RETURNING Contact(Id)';\n  Test.setFixedSearchResults(new List<Id>{first.Id});expectIds(rows(query),new Set<Id>{first.Id});Test.startTest();expectIds(rows(query),new Set<Id>{first.Id});Test.setFixedSearchResults(new List<Id>{second.Id});expectIds(rows(query),new Set<Id>{second.Id});Test.stopTest();expectIds(rows(query),new Set<Id>{second.Id});\n }\n @IsTest static void returningLiteralBackslashWhere(){\n  String slash='\\\\';String name='Trail'+slash+'Path';Contact first=new Contact(LastName=name),second=new Contact(LastName='TrailPath');insert new List<Contact>{first,second};Test.setFixedSearchResults(new List<Id>{first.Id,second.Id});\n  String escaped=name.replace(slash,slash+slash);String query='FIND \\'Trail*\\' IN ALL FIELDS RETURNING Contact(Id,LastName WHERE LastName = \\''+escaped+'\\')';\n  List<SObject> actual=rows(query);expectIds(actual,new Set<Id>{first.Id});System.assertEquals(name,actual[0].get('LastName'));\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSearchState53.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>53.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"53.0\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 5 || got.Passed != 5 || got.Skipped != 0 || got.Failed != 0 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("search test-context contracts failed: %s", data)
	}
}
