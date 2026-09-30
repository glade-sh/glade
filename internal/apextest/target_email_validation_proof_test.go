package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestTargetEmailValidationSF230(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTargetEmail62Assertions.cls"), `@IsTest private class GladeTargetEmail62Assertions {
 private static Map<String,Object> send(Contact target, Id templateId) {
  Map<String,Object> out=new Map<String,Object>{'callerEmail'=>target.Email,'storedEmail'=>[SELECT Email FROM Contact WHERE Id=:target.Id].Email};
  Messaging.SingleEmailMessage mail=new Messaging.SingleEmailMessage();
  mail.setTargetObjectId(target.Id); mail.setTemplateId(templateId); mail.setSaveAsActivity(false);
  try {
   Messaging.SendEmailResult result=Messaging.sendEmail(new Messaging.Email[]{mail},false)[0];
   out.put('success',result.isSuccess()); List<Object> errors=new List<Object>();
   for(Messaging.SendEmailError error:result.getErrors()) errors.add(new Map<String,Object>{'status'=>String.valueOf(error.getStatusCode()),'message'=>error.getMessage(),'targetIsExpected'=>error.getTargetObjectId()==target.Id,'targetIsNull'=>error.getTargetObjectId()==null});
   out.put('errors',errors);
  }catch(Exception error){out.put('exceptionType',error.getTypeName());out.put('exceptionMessage',error.getMessage());}
  return out;
 }
 @IsTest static void assertValidAndClearedTargetEmail() {
  Map<String,Object> out=new Map<String,Object>();
  try {
   EmailTemplate template=new EmailTemplate(Name='Owned Active Target Email62',DeveloperName='GladeTargetEmail62AssertOwned',IsActive=true,FolderId=UserInfo.getUserId(),TemplateType='text',Subject='Owned target email observer',Body='Owned message');
   insert template;
   System.runAs(new User(Id=UserInfo.getUserId())) {
   Contact target=new Contact(LastName='Owned target',Email='owned@example.invalid'); insert target;
   out.put('valid',send(target,template.Id));
   target.Email=null; update target;
   out.put('cleared',send(target,template.Id));
   }
  }catch(Exception error){out.put('setupExceptionType',error.getTypeName());out.put('setupExceptionMessage',error.getMessage());}
  System.assertEquals(new Set<String>{'valid','cleared'},out.keySet());
  Map<String,Object> valid=(Map<String,Object>)out.get('valid');
  System.assertEquals(new Set<String>{'callerEmail','storedEmail','success','errors'},valid.keySet());
  System.assertEquals('owned@example.invalid',valid.get('callerEmail'));
  System.assertEquals('owned@example.invalid',valid.get('storedEmail'));
  System.assertEquals(true,valid.get('success'));
  System.assertEquals(0,((List<Object>)valid.get('errors')).size());
  Map<String,Object> cleared=(Map<String,Object>)out.get('cleared');
  System.assertEquals(new Set<String>{'callerEmail','storedEmail','success','errors'},cleared.keySet());
  System.assertEquals(null,cleared.get('callerEmail'));
  System.assertEquals(null,cleared.get('storedEmail'));
  System.assertEquals(false,cleared.get('success'));
  List<Object> errors=(List<Object>)cleared.get('errors');
  System.assertEquals(1,errors.size());
  Map<String,Object> error=(Map<String,Object>)errors[0];
  System.assertEquals(new Set<String>{'status','message','targetIsExpected','targetIsNull'},error.keySet());
  System.assertEquals('INVALID_EMAIL_ADDRESS',error.get('status'));
  System.assertEquals('The target object\'s email address "null" is not valid',error.get('message'));
  System.assertEquals(true,error.get('targetIsExpected'));
  System.assertEquals(false,error.get('targetIsNull'));
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTargetEmail62Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("SF230: %s", b)
	}
}

func TestTargetEmailRecipientOptionsAPI62(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "40.0", "packageDirectories": [{"path": "force-app", "default": true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTargetEmail62OptionsAssertions.cls"), `@IsTest private class GladeTargetEmail62OptionsAssertions {
 private static Map<String,Object> send(Contact target, Id templateId, Boolean recipientOption, Boolean extraRecipient) {
  Map<String,Object> out=new Map<String,Object>{'callerEmail'=>target.Email,'storedEmail'=>[SELECT Email FROM Contact WHERE Id=:target.Id].Email};
  Messaging.SingleEmailMessage mail=new Messaging.SingleEmailMessage();
  mail.setTargetObjectId(target.Id); mail.setTemplateId(templateId); mail.setSaveAsActivity(false);
  if(recipientOption!=null)mail.setTreatTargetObjectAsRecipient(recipientOption);
  if(extraRecipient)mail.setToAddresses(new List<String>{'explicit@example.invalid'});
  out.put('isRecipientOption',mail.isTreatTargetObjectAsRecipient());
  try {
   Messaging.SendEmailResult result=Messaging.sendEmail(new Messaging.Email[]{mail},false)[0];
   out.put('success',result.isSuccess()); List<Object> errors=new List<Object>();
   for(Messaging.SendEmailError error:result.getErrors()) errors.add(new Map<String,Object>{'status'=>String.valueOf(error.getStatusCode()),'message'=>error.getMessage(),'targetIsExpected'=>error.getTargetObjectId()==target.Id,'targetIsNull'=>error.getTargetObjectId()==null});
   out.put('errors',errors);
  }catch(Exception error){out.put('exceptionType',error.getTypeName());out.put('exceptionMessage',error.getMessage());}
  return out;
 }
 private static void verify(Map<String,Object> actual,String email,Boolean recipient,Boolean success) {
  System.assertEquals(new Set<String>{'callerEmail','storedEmail','isRecipientOption','success','errors'},actual.keySet());
  System.assertEquals(email,actual.get('callerEmail'));System.assertEquals(email,actual.get('storedEmail'));
  System.assertEquals(recipient,actual.get('isRecipientOption'));System.assertEquals(success,actual.get('success'));
  List<Object> errors=(List<Object>)actual.get('errors');System.assertEquals(success?0:1,errors.size());
  if(!success){
   Map<String,Object> error=(Map<String,Object>)errors[0];
   System.assertEquals(new Set<String>{'targetIsNull','targetIsExpected','message','status'},error.keySet());
   System.assertEquals(false,error.get('targetIsNull'));System.assertEquals(true,error.get('targetIsExpected'));
   System.assertEquals('INVALID_EMAIL_ADDRESS',error.get('status'));
   System.assertEquals('The target object\'s email address "null" is not valid',error.get('message'));
  }
 }
 @IsTest static void assertTargetRecipientOptions() {
  Map<String,Object> out=new Map<String,Object>();
  try {
   EmailTemplate template=new EmailTemplate(Name='Owned Active Target Email62',DeveloperName='GladeTargetEmail62OptionsAssertOwned',IsActive=true,FolderId=UserInfo.getUserId(),TemplateType='text',Subject='Owned target email observer',Body='Owned message');
   insert template;
   System.runAs(new User(Id=UserInfo.getUserId())) {
   Contact target=new Contact(LastName='Owned target',Email='owned@example.invalid'); insert target;
   out.put('validOmitted',send(target,template.Id,null,false));
   target.Email=null; update target;
   out.put('clearedOmitted',send(target,template.Id,null,false));
   out.put('clearedOmittedWithTo',send(target,template.Id,null,true));
   out.put('clearedFalseWithTo',send(target,template.Id,false,true));
   out.put('clearedTrueWithTo',send(target,template.Id,true,true));
   }
  }catch(Exception error){out.put('setupExceptionType',error.getTypeName());out.put('setupExceptionMessage',error.getMessage());}
  System.assertEquals(new Set<String>{'validOmitted','clearedOmitted','clearedOmittedWithTo','clearedFalseWithTo','clearedTrueWithTo'},out.keySet());
  verify((Map<String,Object>)out.get('validOmitted'),'owned@example.invalid',true,true);
  verify((Map<String,Object>)out.get('clearedOmitted'),null,true,false);
  verify((Map<String,Object>)out.get('clearedOmittedWithTo'),null,true,false);
  verify((Map<String,Object>)out.get('clearedFalseWithTo'),null,false,true);
  verify((Map<String,Object>)out.get('clearedTrueWithTo'),null,true,false);
 }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTargetEmail62OptionsAssertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("API62 targetrecipient: %s", b)
	}
}
