trigger RpModeTrigger on RpMaster2__c (after insert) {
  if (RpModeHold.child == null) return;
  try { RpChild__c x=[SELECT Id,Name,Master__c FROM RpChild__c WHERE Id=:RpModeHold.child]; x.Name='trg'; update x; RpModeHold.res='ok'; }
  catch(Exception e) { RpModeHold.res=e.getTypeName()+'|'+e.getMessage().replaceAll('\\s+',' '); }
}
