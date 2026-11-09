public with sharing class DiskPermissionObserver67 {
 public static String observe() {
  Map<String,Object> row=new Map<String,Object>{
   'profile'=>[SELECT Name FROM Profile WHERE Id=:UserInfo.getProfileId()].Name,
   'assignmentCount'=>[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId()],
   'probeAssignments'=>[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSet.Name='DiskRuntimeLookupProbe'],
   'groupCount'=>[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSetGroupId!=null]};
  Map<String,Schema.SObjectField> fields=Schema.getGlobalDescribe().get('LookupScopeChild__c').getDescribe().fields.getMap();
  for(String key:fields.keySet()) if(fields.get(key).getDescribe().getName()=='Parent__c') {
   Schema.DescribeFieldResult f=fields.get(key).getDescribe();
   row.put('readable',f.isAccessible());row.put('createable',f.isCreateable());row.put('updateable',f.isUpdateable());
  }
  return JSON.serialize(row);
 }
}
