trigger FlowBatchRefireTrigger on FlowCoreAORecord__c (after update) {
    FlowBatchRefireLog.log.add(Trigger.new.size()+':'+Trigger.new[0].Name+':'+Trigger.new[0].Unit__c);
    for (FlowCoreAORecord__c r : Trigger.new) {
        if (r.Name == 'Synthetic 0' && r.Unit__c == 'updated') {
            List<FlowCoreAORecord__c> other=[SELECT Id,Name FROM FlowCoreAORecord__c WHERE Name='Synthetic 1'];
            for (FlowCoreAORecord__c o : other) o.Name='Edited by trigger';
            update other;
        }
    }
}
