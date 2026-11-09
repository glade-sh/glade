trigger SetupTrigger on Account (before insert) {
 for (Account item : Trigger.new) { item.TriggerValue__c = item.SetupFlag__c; }
}
