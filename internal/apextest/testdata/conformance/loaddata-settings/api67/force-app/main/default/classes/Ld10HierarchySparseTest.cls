@IsTest private class Ld10HierarchySparseTest {
 @IsTest static void observe() {
  delete [SELECT Id FROM HierPref__c];
  Profile profile=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User reader=new User(Alias='lds',Email='reader@example.test',EmailEncodingKey='UTF-8',LastName='Sparse',
   LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=profile.Id,
   Username='lds.'+String.valueOf(Crypto.getRandomLong())+'.'+UserInfo.getOrganizationId()+'@example.test');
  insert reader;
  insert new HierPref__c(SetupOwnerId=UserInfo.getOrganizationId(),Value__c='org',Num__c=1,Flag__c=false,Note__c='org-note');
  insert new HierPref__c(SetupOwnerId=profile.Id,Value__c='profile');
  Map<String,Object> o=new Map<String,Object>();
  System.runAs(reader) {
   HierPref__c i=HierPref__c.getInstance();
   o.put('inst',val(i));
   o.put('instUser',inst(reader.Id));
   o.put('instProfile',inst(profile.Id));
   o.put('valuesProfile',vals(profile.Id));
   o.put('org',val(HierPref__c.getOrgDefaults()));
   o.put('instIdNull',i.Id==null);
   o.put('instOwnerUser',i.SetupOwnerId==reader.Id);
  }
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
 static String val(HierPref__c x) { return x==null?'null':x.Value__c+'/'+String.valueOf(x.Num__c)+'/'+String.valueOf(x.Flag__c)+'/'+x.Note__c; }
 static String inst(Id owner) { try { return val(HierPref__c.getInstance(owner)); } catch(Exception e) { return e.getTypeName(); } }
 static String vals(Id owner) { try { return val(HierPref__c.getValues(owner)); } catch(Exception e) { return e.getTypeName(); } }
}
