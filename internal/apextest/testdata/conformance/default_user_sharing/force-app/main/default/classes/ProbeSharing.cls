public with sharing class ProbeSharing {
 public static String observe(Id itemId) {
  Integer visible=[SELECT count() FROM ProbeItem__c WHERE Id=:itemId];
  UserRecordAccess a=[SELECT RecordId, HasReadAccess, HasEditAccess FROM UserRecordAccess WHERE UserId=:UserInfo.getUserId() AND RecordId=:itemId];
  String result='ok';
  try { update new ProbeItem__c(Id=itemId,Name='Changed'); } catch(DmlException e) {result=String.valueOf(e.getDmlType(0));}
  return String.valueOf(visible)+'|'+a.HasReadAccess+'|'+a.HasEditAccess+'|'+result;
 }
}
