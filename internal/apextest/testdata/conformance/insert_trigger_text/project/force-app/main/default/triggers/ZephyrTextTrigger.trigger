trigger ZephyrTextTrigger on Zephyr__c (before insert, after insert) {
    for (Zephyr__c row : Trigger.new) {
        if (Trigger.isBefore) {
            ZephyrTextProbe.record('before', row);
            row.TriggerWrite__c = '  gust  ';
            ZephyrTextProbe.record('after-assignment', row);
        } else {
            ZephyrTextProbe.record('after', row);
        }
    }
}
