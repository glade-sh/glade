public inherited sharing class SettingQuerySlice {
 public static Map<String,Object> hierarchy() {
  Id organization=UserInfo.getOrganizationId();
  List<Id> owners=new List<Id>{UserInfo.getUserId(),UserInfo.getProfileId(),organization};
  List<SObject> rows=Database.queryWithBinds('SELECT SetupOwnerId,Enabled__c FROM RoutePreference__c WHERE SetupOwnerId IN :owners',new Map<String,Object>{'owners'=>owners},AccessLevel.SYSTEM_MODE);
  return new Map<String,Object>{'rows'=>rows.size(),'value'=>rows.isEmpty()?null:rows[0].get('Enabled__c'),
   'orgOwner'=>!rows.isEmpty() && (Id)rows[0].get('SetupOwnerId')==organization};
 }
 public static Map<String,Object> listRecord() {
  List<SObject> rows=Database.queryWithBinds('SELECT Name FROM RouteSchedule__c WHERE Name = :name',new Map<String,Object>{'name'=>'SharedEvening'},AccessLevel.SYSTEM_MODE);
  return new Map<String,Object>{'rows'=>rows.size(),'name'=>rows.isEmpty()?null:rows[0].get('Name')};
 }
}
