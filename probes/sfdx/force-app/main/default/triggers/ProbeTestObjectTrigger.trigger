trigger ProbeTestObjectTrigger on ProbeTestObject__c (before insert, before update) {
    for (ProbeTestObject__c rec : Trigger.new) {
        rec.Triggered__c = true;
    }
}
