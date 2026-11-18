trigger RollChildTrigger on RollChild__c (after insert, after update, after delete, after undelete) {
 RollTriggerLog.capture(Trigger.operationType,Trigger.new,Trigger.old);
}
