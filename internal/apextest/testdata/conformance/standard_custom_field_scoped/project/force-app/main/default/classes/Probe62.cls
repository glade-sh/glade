@IsTest private class Probe62 {
 static String run(Boolean scoped, Boolean partial, Boolean runAsUser, String field) {
  User u; if(runAsUser) { Profile p=[SELECT Id FROM Profile WHERE Name='Standard User']; String k=String.valueOf(Math.abs(Crypto.getRandomInteger())); u=new User(Alias='su'+k.left(5),Email='su@example.invalid',EmailEncodingKey='UTF-8',LastName='T',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',UserName='su'+k+'@example.invalid.glade'); insert u; }
  Id ps=[SELECT Id FROM PermissionSet WHERE Name='Probe_Access'].Id;
  if(runAsUser) { String result; System.runAs(u) { result=doit(scoped,partial,field,ps); } return result; } return doit(scoped,partial,field,ps);
 }
 static String doit(Boolean scoped, Boolean partial, String field, Id ps) { Account a=new Account(Name='x'); a.put(field,'v'); try { AccessLevel level=scoped?AccessLevel.USER_MODE.withPermissionSetId(ps):AccessLevel.USER_MODE; Database.SaveResult r=Database.insert(a,!partial,level); return r.isSuccess()?'ok':'fail '+r.getErrors()[0].getStatusCode()+':'+r.getErrors()[0].getMessage(); } catch(Exception e) {return e.getTypeName()+':'+e.getMessage();} }
 @IsTest static void scopedPartial() {System.assert(false,run(true,true,true,'Probe_B__c'));}
 @IsTest static void scopedAll() {System.assert(false,run(true,false,true,'Probe_B__c'));}
 @IsTest static void scopedGrant() {System.assert(false,run(true,true,true,'Probe_A__c'));}
 @IsTest static void adminPartial() {System.assert(false,run(false,true,false,'Probe_B__c'));}
 @IsTest static void adminScoped() {System.assert(false,run(true,true,false,'Probe_B__c'));}
}