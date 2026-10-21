@IsTest private class SettingOwnerQueryTest {
 private static User actor(String label) {
  User value=new User(Alias=label,Email='setting@example.test',EmailEncodingKey='UTF-8',LastName='Setting '+label,
   LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=UserInfo.getProfileId(),
   Username='setting.'+label+'.'+UserInfo.getOrganizationId()+'.'+String.valueOf(Datetime.now().getTime())+'@example.test');
  insert value;
  return value;
 }
 private static void writeDefault(Boolean enabled) {
  RoutePreference__c value=RoutePreference__c.getOrgDefaults();
  if(value.Id==null) value.SetupOwnerId=UserInfo.getOrganizationId();
  value.Enabled__c=enabled;
  Database.upsert(value,true,AccessLevel.SYSTEM_MODE);
 }
 private static Map<String,Object> readPreference() {
  Id organization=UserInfo.getOrganizationId();
  List<Id> ownerIds=new List<Id>{UserInfo.getUserId(),UserInfo.getProfileId(),organization};
  List<SObject> rows=Database.query('SELECT SetupOwnerId,Enabled__c FROM RoutePreference__c WHERE SetupOwnerId IN :ownerIds',AccessLevel.SYSTEM_MODE);
  Map<Id,SObject> byOwner=new Map<Id,SObject>();
  for(SObject row:rows) byOwner.put((Id)row.get('SetupOwnerId'),row);
  SObject selected;
  for(Id owner:ownerIds) {
   selected=byOwner.get(owner);
   if(selected!=null) break;
  }
  return new Map<String,Object>{'rows'=>rows.size(),'queryValue'=>selected?.get('Enabled__c'),
   'orgValue'=>RoutePreference__c.getOrgDefaults().Enabled__c,'instanceValue'=>RoutePreference__c.getInstance().Enabled__c,
   'selectedOrg'=>selected!=null && (Id)selected.get('SetupOwnerId')==organization};
 }
 private static void observe(String method,Map<String,Object> result) {
  System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(result));
 }
 @IsTest static void orgFalseSecondUser() {
  User writer=actor('falseA');User reader=actor('falseB');
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(writer) { writeDefault(false);result.put('writer',readPreference()); }
  System.runAs(reader) { result.put('reader',readPreference()); }
  observe('orgFalseSecondUser',result);
 }
 @IsTest static void orgRoundTrip() {
  User writer=actor('roundA');User reader=actor('roundB');
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(writer) { writeDefault(false);result.put('off',readPreference());writeDefault(true); }
  System.runAs(reader) { result.put('reader',readPreference()); }
  observe('orgRoundTrip',result);
 }
 @IsTest static void userOverrideWins() {
  User writer=actor('overA');User reader=actor('overB');
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(writer) {
   writeDefault(false);
   insert new RoutePreference__c(SetupOwnerId=reader.Id,Enabled__c=true);
  }
  System.runAs(reader) { result.put('reader',readPreference()); }
  observe('userOverrideWins',result);
 }
 @IsTest static void listNameOtherUser() {
  User writer=actor('listA');User reader=actor('listB');
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(writer) { insert new RouteSchedule__c(Name='EveningRoute'); }
  System.runAs(reader) {
   String expected='EveningRoute';
   List<SObject> rows=Database.query('SELECT Name FROM RouteSchedule__c WHERE Name = :expected',AccessLevel.SYSTEM_MODE);
   result.put('rows',rows.size());
   result.put('name',rows.isEmpty()?null:rows[0].get('Name'));
   result.put('matches',!rows.isEmpty() && expected.equalsIgnoreCase(String.valueOf(rows[0].get('Name'))));
   result.put('cachedName',RouteSchedule__c.getInstance(expected)?.Name);
  }
  observe('listNameOtherUser',result);
 }
 private static User standardReader(String label) {
  User reader=actor(label);
  reader.ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id;
  update reader;
  return reader;
 }
 @IsTest static void withSharedHierarchy() {
  User writer=actor('sharedA');User reader=standardReader('sharedB');
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(writer) { writeDefault(false); }
  System.runAs(reader) {
   result.put('shared',SharedSettingReader.hierarchy());
   result.put('unshared',UnsharedSettingReader.hierarchy());
   result.put('orgValue',RoutePreference__c.getOrgDefaults().Enabled__c);
   result.put('instanceValue',RoutePreference__c.getInstance().Enabled__c);
  }
  observe('withSharedHierarchy',result);
 }
 @IsTest static void withSharedList() {
  User writer=actor('listShA');User reader=standardReader('listShB');
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(writer) { insert new RouteSchedule__c(Name='SharedEvening'); }
  System.runAs(reader) {
   result.put('shared',SharedSettingReader.listRecord());
   result.put('unshared',UnsharedSettingReader.listRecord());
   result.put('cachedName',RouteSchedule__c.getInstance('SharedEvening')?.Name);
  }
  observe('withSharedList',result);
 }

}
