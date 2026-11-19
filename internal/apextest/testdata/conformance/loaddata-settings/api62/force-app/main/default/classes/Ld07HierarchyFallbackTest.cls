@IsTest private class Ld07HierarchyFallbackTest {
 @IsTest static void observe() {
  delete [SELECT Id FROM HierPref__c];
  Profile profile=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User reader=new User(Alias='ldu',Email='reader@example.test',EmailEncodingKey='UTF-8',LastName='Load',
   LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=profile.Id,
   Username='ldu.'+String.valueOf(Crypto.getRandomLong())+'.'+UserInfo.getOrganizationId()+'@example.test');
  insert reader;
  insert new HierPref__c(SetupOwnerId=UserInfo.getOrganizationId(),Value__c='org',Num__c=1,Flag__c=false);
  insert new HierPref__c(SetupOwnerId=profile.Id,Value__c='profile',Num__c=2,Flag__c=true);
  insert new HierPref__c(SetupOwnerId=reader.Id,Value__c='user',Num__c=3,Flag__c=false);
  Map<String,Object> o=new Map<String,Object>();
  System.runAs(reader) { o.put('withUser',shape(profile.Id)); }
  delete [SELECT Id FROM HierPref__c WHERE SetupOwnerId=:reader.Id];
  System.runAs(reader) { o.put('withProfile',shape(profile.Id)); }
  delete [SELECT Id FROM HierPref__c WHERE SetupOwnerId=:profile.Id];
  System.runAs(reader) { o.put('withOrg',shape(profile.Id)); }
  delete [SELECT Id FROM HierPref__c WHERE SetupOwnerId=:UserInfo.getOrganizationId()];
  System.runAs(reader) { o.put('withNone',shape(profile.Id)); }
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
 static String val(HierPref__c row) { return row==null?'null':row.Value__c+'/'+String.valueOf(row.Num__c)+'/'+row.Flag__c; }
 static String byId(Id owner) { try { return val(HierPref__c.getValues(owner)); } catch(Exception e) { return e.getTypeName(); } }
 static String instById(Id owner) { try { return val(HierPref__c.getInstance(owner)); } catch(Exception e) { return e.getTypeName(); } }
 static String shape(Id profileId) {
  return 'inst='+val(HierPref__c.getInstance())+'|org='+val(HierPref__c.getOrgDefaults())
   +'|user='+byId(UserInfo.getUserId())+'|prof='+byId(profileId)+'|own='+byId(UserInfo.getOrganizationId())
   +'|instU='+instById(UserInfo.getUserId())+'|instP='+instById(profileId);
 }
}
