package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// SF218 admits the API65 implicit Text custom-name length and DML boundary.
func TestRunCustomTextNameLengthAPI65Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace":"","sourceApiVersion":"65.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTextName65Assertions.cls"), `@IsTest private class GladeTextName65Assertions {
 private static Map<String,Object> attempt(String input) {
  GladeTextName65Assert__c row=new GladeTextName65Assert__c(Name=input);
  Map<String,Object> result=new Map<String,Object>{'input'=>input,'inputLength'=>input.length(),'callerBefore'=>row.Name};
  try {
   insert row;
   result.put('inserted',true);
   GladeTextName65Assert__c stored=[SELECT Name FROM GladeTextName65Assert__c WHERE Id=:row.Id];
   result.put('stored',stored.Name);result.put('storedLength',stored.Name.length());
  }catch(DmlException error) {
   result.put('inserted',false);result.put('errorType',error.getTypeName());result.put('errorMessage',error.getMessage());
   result.put('statusCode',String.valueOf(error.getDmlStatusCode(0)));
   result.put('errorFields',error.getDmlFieldNames(0));
  }
  result.put('callerAfter',row.Name);result.put('callerIdIsNull',row.Id==null);
  return result;
 }
 @IsTest static void assertImplicitTextNameLength() {
  Map<String,Object> result=new Map<String,Object>{'describeLength'=>GladeTextName65Assert__c.Name.getDescribe().getLength()};
  result.put('length80',attempt('AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'));
  result.put('length83',attempt('BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB'));
  result.put('storedCount',[SELECT COUNT() FROM GladeTextName65Assert__c]);
  System.assertEquals(new Set<String>{'storedCount','length83','length80','describeLength'},result.keySet());
  System.assertEquals(1,result.get('storedCount'),'storedCount');
  { Map<String,Object> actual=(Map<String,Object>)result.get('length83');
   System.assertEquals(new Set<String>{'callerIdIsNull','callerAfter','errorFields','statusCode','errorMessage','errorType','inserted','callerBefore','inputLength','input'},actual.keySet());
   System.assertEquals(true,actual.get('callerIdIsNull'),'length83.callerIdIsNull');
   System.assertEquals('BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB',actual.get('callerAfter'),'length83.callerAfter');
   System.assertEquals(new List<String>{'Name'},actual.get('errorFields'),'length83.errorFields');
   System.assertEquals('STRING_TOO_LONG',actual.get('statusCode'),'length83.statusCode');
   System.assertEquals('Insert failed. First exception on row 0; first error: STRING_TOO_LONG, Owned Text Name: data value too large: BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB (max length=80): [Name]',actual.get('errorMessage'),'length83.errorMessage');
   System.assertEquals('System.DmlException',actual.get('errorType'),'length83.errorType');
   System.assertEquals(false,actual.get('inserted'),'length83.inserted');
   System.assertEquals('BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB',actual.get('callerBefore'),'length83.callerBefore');
   System.assertEquals(83,actual.get('inputLength'),'length83.inputLength');
   System.assertEquals('BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB',actual.get('input'),'length83.input');
  }
  { Map<String,Object> actual=(Map<String,Object>)result.get('length80');
   System.assertEquals(new Set<String>{'callerIdIsNull','callerAfter','storedLength','stored','inserted','callerBefore','inputLength','input'},actual.keySet());
   System.assertEquals(false,actual.get('callerIdIsNull'),'length80.callerIdIsNull');
   System.assertEquals('AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA',actual.get('callerAfter'),'length80.callerAfter');
   System.assertEquals(80,actual.get('storedLength'),'length80.storedLength');
   System.assertEquals('AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA',actual.get('stored'),'length80.stored');
   System.assertEquals(true,actual.get('inserted'),'length80.inserted');
   System.assertEquals('AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA',actual.get('callerBefore'),'length80.callerBefore');
   System.assertEquals(80,actual.get('inputLength'),'length80.inputLength');
   System.assertEquals('AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA',actual.get('input'),'length80.input');
  }
  System.assertEquals(80,result.get('describeLength'),'describeLength');
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTextName65Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/GladeTextName65Assert__c/GladeTextName65Assert__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Owned Text Name</label><nameField><label>Owned Text Name</label><type>Text</type></nameField><pluralLabel>Owned Text Names</pluralLabel><sharingModel>ReadWrite</sharingModel></CustomObject>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Failed != 0 || got.Errors != 0 {
		data, _ := json.Marshal(run)
		t.Fatalf("SF218 custom Text name length: %s", data)
	}
}
