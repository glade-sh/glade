package apextest

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// Source metadata names the child relationship without an Apex __r suffix.
// Keep inference, child filtering, typed iteration and cached record views on
// the same project-loading path used by ordinary standalone Apex projects.
func TestMetadataChildIteratorProjection(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, setup := range []int{0, 1, 2} {
			name := api + "/direct"
			if setup != 0 {
				name = api + "/warmSetup"
				if setup == 2 {
					name = api + "/constructorWarmSetup"
				}
			}
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				base := filepath.Join(root, "force-app/main/default")
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				for _, object := range []string{"RuleBook", "RuleEntry"} {
					writeFile(t, filepath.Join(base, "objects", object+"__mdt", object+"__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>`+object+`</label><pluralLabel>`+object+` records</pluralLabel><visibility>Public</visibility></CustomObject>`)
					writeFile(t, filepath.Join(base, "objects", object+"__mdt", "fields/IsActive__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>IsActive__c</fullName><label>Active</label><type>Checkbox</type><defaultValue>false</defaultValue><fieldManageability>DeveloperControlled</fieldManageability></CustomField>`)
				}
				writeFile(t, filepath.Join(base, "objects/RuleEntry__mdt/fields/Book__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Book__c</fullName><label>Book</label><type>MetadataRelationship</type><referenceTo>RuleBook__mdt</referenceTo><relationshipLabel>Rules</relationshipLabel><relationshipName>Rules</relationshipName><required>true</required><fieldManageability>DeveloperControlled</fieldManageability></CustomField>`)
				writeFile(t, filepath.Join(base, "objects/RuleEntry__mdt/fields/Position__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Position__c</fullName><label>Position</label><type>Number</type><precision>3</precision><scale>0</scale><fieldManageability>DeveloperControlled</fieldManageability></CustomField>`)
				for _, name := range []string{"Example", "Empty"} {
					writeFile(t, filepath.Join(base, "customMetadata", "RuleBook."+name+".md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>`+name+`</label><protected>false</protected><values><field>IsActive__c</field><value xsi:type="xsd:boolean">true</value></values></CustomMetadata>`)
				}
				for _, record := range []struct{ name, active, position string }{
					{"First", "true", "1"},
					{"Inactive", "false", "2"},
				} {
					writeFile(t, filepath.Join(base, "customMetadata", "RuleEntry."+record.name+".md-meta.xml"), `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>`+record.name+`</label><protected>false</protected><values><field>Book__c</field><value xsi:type="xsd:string">Example</value></values><values><field>IsActive__c</field><value xsi:type="xsd:boolean">`+record.active+`</value></values><values><field>Position__c</field><value xsi:type="xsd:double">`+record.position+`</value></values></CustomMetadata>`)
				}
				proofSource := metadataChildIteratorProofSource
				if setup != 0 {
					proofSource = strings.Replace(proofSource, "@IsTest private class MetadataChildIteratorProof {", `@IsTest private class MetadataChildIteratorProof {
 @TestSetup static void warmSetup() {
  System.assertEquals('First',RuleCatalog.entries.get('Example').Rules__r.iterator().next().DeveloperName);
  insert new Account(Name='Setup anchor');
 }`, 1)
				}
				if setup == 2 {
					proofSource = strings.Replace(proofSource, "System.assertEquals('First',RuleCatalog.entries.get('Example').Rules__r.iterator().next().DeveloperName);", "System.assertEquals(true,new RuleConsumer().available);", 1)
				}
				for name, source := range map[string]string{
					"RuleCatalog":                metadataChildIteratorCatalogSource,
					"RuleConsumer":               `public class RuleConsumer { public Boolean available=RuleCatalog.entries.get('Example').Rules__r.iterator().hasNext(); }`,
					"MetadataChildIteratorProof": proofSource,
				} {
					writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
					writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
				}
				run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
				if got := run.Summary(); got.Total != 5 || got.Passed != 5 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
					data, _ := json.Marshal(run)
					t.Fatalf("metadata child iterator projection: %s", data)
				}
			})
		}
	}
}

const metadataChildIteratorCatalogSource = `public class RuleCatalog {
 public static Integer loads=0;
 public static Map<String,RuleBook__mdt> entries {
  get {
   if(entries==null) entries=loadEntries();
   return entries;
  }
  private set;
 }
 private static Map<SObjectField,DescribeFieldResult> descriptions=new Map<SObjectField,DescribeFieldResult>();
 private static String fieldName(SObjectField token) {
  DescribeFieldResult description=descriptions.get(token);
  if(description==null){description=token.getDescribe();descriptions.put(token,description);}
  return description.name;
 }
 public static String queryText() {
  List<String> parentFields=new List<String>{fieldName(RuleBook__mdt.DeveloperName),fieldName(RuleBook__mdt.IsActive__c)};
  List<String> childFields=new List<String>{fieldName(RuleEntry__mdt.DeveloperName),fieldName(RuleEntry__mdt.Book__c),fieldName(RuleEntry__mdt.IsActive__c),fieldName(RuleEntry__mdt.Position__c)};
  return 'SELECT '+String.join(parentFields,',')+',('+'SELECT '+String.join(childFields,',')+' FROM Rules__r WHERE '+fieldName(RuleEntry__mdt.IsActive__c)+'=true) FROM RuleBook__mdt WHERE '+fieldName(RuleBook__mdt.IsActive__c)+'=true';
 }
 private static Map<String,RuleBook__mdt> loadEntries() {
  Map<String,RuleBook__mdt> result=new Map<String,RuleBook__mdt>();
  for(RuleBook__mdt book:Database.query(queryText(),AccessLevel.SYSTEM_MODE)) result.put(book.DeveloperName,book);
  loads++;
  return result;
 }
}`

const metadataChildIteratorProofSource = `@IsTest private class MetadataChildIteratorProof {
 private static void assertFirst(RuleBook__mdt book) {
  System.assertEquals(1,book.Rules__r.size(),'one active child');
  Iterator<RuleEntry__mdt> cursor=book.Rules__r.iterator();
  System.assert(cursor.hasNext());
  RuleEntry__mdt entry=cursor.next();
  System.assertEquals('First',entry.DeveloperName);
  System.assertEquals(book.Id,entry.Book__c,'metadata name resolves to parent identity');
  System.assertEquals(true,entry.IsActive__c);
  System.assertEquals(1,entry.Position__c);
  System.assert(!cursor.hasNext());
 }
 @IsTest static void directDynamicQuery() {
  List<RuleBook__mdt> books=Database.query(RuleCatalog.queryText()+' AND DeveloperName=\'Example\'',AccessLevel.SYSTEM_MODE);
  System.assertEquals(1,books.size());
  assertFirst(books[0]);
 }
 @IsTest static void typedMapLookup() {
  Map<String,RuleBook__mdt> books=new Map<String,RuleBook__mdt>();
  for(RuleBook__mdt book:Database.query(RuleCatalog.queryText(),AccessLevel.SYSTEM_MODE)) books.put(book.DeveloperName,book);
  System.assertEquals('First',books.get('Example').Rules__r.iterator().next().DeveloperName);
  assertFirst(books.get('Example'));
 }
 @IsTest static void staticCachedMapLookup() {
  System.assertEquals('First',RuleCatalog.entries.get('Example').Rules__r.iterator().next().DeveloperName);
  assertFirst(RuleCatalog.entries.get('Example'));
  System.assertEquals(1,RuleCatalog.loads,'second property read retains the same query cache');
 }
 @IsTest static void inactiveChildControl() {
  List<RuleBook__mdt> allChildren=Database.query('SELECT DeveloperName,(SELECT DeveloperName,IsActive__c FROM Rules__r) FROM RuleBook__mdt WHERE DeveloperName=\'Example\'',AccessLevel.SYSTEM_MODE);
  System.assertEquals(2,allChildren[0].Rules__r.size(),'inactive child exists before filtering');
  Set<String> names=new Set<String>();
  for(RuleEntry__mdt entry:allChildren[0].Rules__r) names.add(entry.DeveloperName);
  System.assertEquals(new Set<String>{'First','Inactive'},names);
  RuleBook__mdt parent=[SELECT Id FROM RuleBook__mdt WHERE DeveloperName='Example'];
  Id parentId=parent.Id;
  List<RuleEntry__mdt> selected=[SELECT DeveloperName FROM RuleEntry__mdt WHERE Book__c=:parentId AND IsActive__c=true];
  System.assertEquals(1,selected.size());
  System.assertEquals('First',selected[0].DeveloperName);
 }
 @IsTest static void emptyChildControl() {
  RuleBook__mdt book=RuleCatalog.entries.get('Empty');
  System.assertNotEquals(null,book,'parent exists');
  System.assertEquals(0,book.Rules__r.size());
  System.assert(!book.Rules__r.iterator().hasNext());
 }
}`
