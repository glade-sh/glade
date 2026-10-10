package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSystemFieldDynamicPutSF240(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSystemPut62Split.cls"), `@IsTest private class GladeSystemPut62Split {
 private static GladeSystemPut62Next__c source() {
  GladeSystemPut62Next__c row=new GladeSystemPut62Next__c(Name='Owned source');insert row;
  return [SELECT Id,Name,IsDeleted,CreatedDate,CreatedById,LastModifiedDate,LastModifiedById,SystemModstamp,OwnerId FROM GladeSystemPut62Next__c WHERE Id=:row.Id];
 }
 private static GladeSystemPut62Next__c query(Id id) {return [SELECT Id,Name,IsDeleted,CreatedDate,CreatedById,LastModifiedDate,LastModifiedById,SystemModstamp,OwnerId FROM GladeSystemPut62Next__c WHERE Id=:id];}
 @IsTest static void assertObservedSystemFieldCopyWrites() {
  GladeSystemPut62Next__c original=source();
  for(String field:new List<String>{'IsDeleted','CreatedDate','CreatedById','LastModifiedDate','LastModifiedById','SystemModstamp'}) {
   Object input=original.get(field);System.assertNotEquals(null,input);
   for(Boolean queried:new List<Boolean>{false,true}) {
    GladeSystemPut62Next__c row=queried?query(original.Id):new GladeSystemPut62Next__c(Name='Owned target');
    Object expected=queried?input:null;System.assertEquals(expected,row.get(field));
    Boolean rejected=false;
    try {row.put(field,input);}catch(Exception error){rejected=true;System.assertEquals('System.SObjectException',error.getTypeName());System.assertEquals('Field '+field+' is not editable',error.getMessage());}
    System.assertEquals(true,rejected);System.assertEquals(expected,row.get(field));
   }
  }
  for(String field:new List<String>{'Id','Name'}) {
   for(Boolean queried:new List<Boolean>{false,true}) {
    GladeSystemPut62Next__c row=queried?query(original.Id):new GladeSystemPut62Next__c(Name='Owned target');
    Object before=queried?original.get(field):(field=='Name'?(Object)'Owned target':null);
    System.assertEquals(before,row.get(field));row.put(field,original.get(field));System.assertEquals(original.get(field),row.get(field));
   }
  }
  GladeSystemPut62Next__c fresh=new GladeSystemPut62Next__c(Name='Owned target');System.assertEquals(null,fresh.OwnerId);fresh.put('OwnerId',original.OwnerId);System.assertEquals(original.OwnerId,fresh.OwnerId);
 }
 @IsTest static void observeCatchCaseBoundary() {
  List<Object> output=new List<Object>();
  Map<String,Object> lower=new Map<String,Object>{'case'=>'lowercaseCatch','innerCaught'=>false,'outerCaught'=>false};
  try {try {new GladeSystemPut62Next__c().put('Derived__c','Injected');}catch(exception error){lower.put('innerCaught',true);lower.put('type',error.getTypeName());lower.put('message',error.getMessage());}}
  catch(Exception error){lower.put('outerCaught',true);lower.put('type',error.getTypeName());lower.put('message',error.getMessage());}
  output.add(lower);
  Map<String,Object> upper=new Map<String,Object>{'case'=>'uppercaseCatch','innerCaught'=>false,'outerCaught'=>false};
  try {try {new GladeSystemPut62Next__c().put('Derived__c','Injected');}catch(Exception error){upper.put('innerCaught',true);upper.put('type',error.getTypeName());upper.put('message',error.getMessage());}}
  catch(Exception error){upper.put('outerCaught',true);upper.put('type',error.getTypeName());upper.put('message',error.getMessage());}
  output.add(upper);System.assert(false,'GLADE_CATCH_CASE62='+JSON.serialize(output));
 }
 @IsTest static void observeQueriedOwnerCopy() {
  GladeSystemPut62Next__c original=source();GladeSystemPut62Next__c row=query(original.Id);
  Map<String,Object> output=new Map<String,Object>{'beforeMatchesInput'=>row.OwnerId==original.OwnerId};
  try {row.put('OwnerId',original.OwnerId);output.put('putSucceeded',true);}catch(Exception error){output.put('putSucceeded',false);output.put('type',error.getTypeName());output.put('message',error.getMessage());}
  output.put('afterMatchesInput',row.OwnerId==original.OwnerId);System.assert(false,'GLADE_QUERIED_OWNER62='+JSON.serialize(output));
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSystemPut62Split.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeSystemPut62Next__c/GladeSystemPut62Next__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned System Put</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Owned System Puts</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeSystemPut62Next__c/fields/Derived__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Derived__c</fullName><formula>Name</formula><label>Derived</label><type>Text</type></CustomField>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, SelectedMethod: "assertObservedSystemFieldCopyWrites"})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("SF240: %s", b)
	}
}

func TestSystemFieldDynamicPutSF242(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSystemPut62FinalAssertions.cls"), `@IsTest private class GladeSystemPut62FinalAssertions {
 private static GladeSystemPut62Next__c source() {
  GladeSystemPut62Next__c row=new GladeSystemPut62Next__c(Name='Owned source');insert row;
  return [SELECT Id,Name,IsDeleted,CreatedDate,CreatedById,LastModifiedDate,LastModifiedById,SystemModstamp,OwnerId FROM GladeSystemPut62Next__c WHERE Id=:row.Id];
 }
 private static GladeSystemPut62Next__c query(Id id) {return [SELECT Id,Name,IsDeleted,CreatedDate,CreatedById,LastModifiedDate,LastModifiedById,SystemModstamp,OwnerId FROM GladeSystemPut62Next__c WHERE Id=:id];}
 @IsTest static void assertObservedSystemFieldCopyWrites() {
  GladeSystemPut62Next__c original=source();
  for(String field:new List<String>{'IsDeleted','CreatedDate','CreatedById','LastModifiedDate','LastModifiedById','SystemModstamp'}) {
   Object input=original.get(field);System.assertNotEquals(null,input);
   for(Boolean queried:new List<Boolean>{false,true}) {
    GladeSystemPut62Next__c row=queried?query(original.Id):new GladeSystemPut62Next__c(Name='Owned target');
    Object expected=queried?input:null;System.assertEquals(expected,row.get(field));
    Boolean rejected=false;
    try {row.put(field,input);}catch(Exception error){rejected=true;System.assertEquals('System.SObjectException',error.getTypeName());System.assertEquals('Field '+field+' is not editable',error.getMessage());}
    System.assertEquals(true,rejected);System.assertEquals(expected,row.get(field));
   }
  }
  for(String field:new List<String>{'Id','Name'}) {
   for(Boolean queried:new List<Boolean>{false,true}) {
    GladeSystemPut62Next__c row=queried?query(original.Id):new GladeSystemPut62Next__c(Name='Owned target');
    Object before=queried?original.get(field):(field=='Name'?(Object)'Owned target':null);
    System.assertEquals(before,row.get(field));row.put(field,original.get(field));System.assertEquals(original.get(field),row.get(field));
   }
  }
  GladeSystemPut62Next__c fresh=new GladeSystemPut62Next__c(Name='Owned target');System.assertEquals(null,fresh.OwnerId);fresh.put('OwnerId',original.OwnerId);System.assertEquals(original.OwnerId,fresh.OwnerId);
 }
 @IsTest static void assertCatchCaseBoundary() {
  List<Object> output=new List<Object>();
  Map<String,Object> lower=new Map<String,Object>{'case'=>'lowercaseCatch','innerCaught'=>false,'outerCaught'=>false};
  try {try {new GladeSystemPut62Next__c().put('Derived__c','Injected');}catch(exception error){lower.put('innerCaught',true);lower.put('type',error.getTypeName());lower.put('message',error.getMessage());}}
  catch(Exception error){lower.put('outerCaught',true);lower.put('type',error.getTypeName());lower.put('message',error.getMessage());}
  output.add(lower);
  Map<String,Object> upper=new Map<String,Object>{'case'=>'uppercaseCatch','innerCaught'=>false,'outerCaught'=>false};
  try {try {new GladeSystemPut62Next__c().put('Derived__c','Injected');}catch(Exception error){upper.put('innerCaught',true);upper.put('type',error.getTypeName());upper.put('message',error.getMessage());}}
  catch(Exception error){upper.put('outerCaught',true);upper.put('type',error.getTypeName());upper.put('message',error.getMessage());}
  output.add(upper);for(Object entry:output){Map<String,Object> row=(Map<String,Object>)entry;System.assertEquals(true,row.get('innerCaught'));System.assertEquals(false,row.get('outerCaught'));System.assertEquals('System.SObjectException',row.get('type'));System.assertEquals('Field Derived__c is not editable',row.get('message'));}
 }
 @IsTest static void assertQueriedOwnerCopy() {
  GladeSystemPut62Next__c original=source();GladeSystemPut62Next__c row=query(original.Id);
  Map<String,Object> output=new Map<String,Object>{'beforeMatchesInput'=>row.OwnerId==original.OwnerId};
  try {row.put('OwnerId',original.OwnerId);output.put('putSucceeded',true);}catch(Exception error){output.put('putSucceeded',false);output.put('type',error.getTypeName());output.put('message',error.getMessage());}
  output.put('afterMatchesInput',row.OwnerId==original.OwnerId);System.assertEquals(new Set<String>{'beforeMatchesInput','putSucceeded','afterMatchesInput'},output.keySet());System.assertEquals(true,output.get('beforeMatchesInput'));System.assertEquals(true,output.get('putSucceeded'));System.assertEquals(true,output.get('afterMatchesInput'));
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeSystemPut62FinalAssertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeSystemPut62Next__c/GladeSystemPut62Next__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned System Put</label><nameField><label>Name</label><type>Text</type></nameField><pluralLabel>Owned System Puts</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeSystemPut62Next__c/fields/Derived__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Derived__c</fullName><formula>Name</formula><label>Derived</label><type>Text</type></CustomField>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 {
		b, _ := json.Marshal(run)
		t.Fatalf("SF242: %s", b)
	}
}
