trigger TfTrigger on Chd__c (before insert, after insert, before update, after update) {
    for (Chd__c n : Trigger.new) {
        if (n.Name != 'TfProbeRow') continue;
        Chd__c o = Trigger.isUpdate ? Trigger.oldMap.get(n.Id) : null;
        TfProbe.record((Trigger.isBefore ? 'before' : 'after') + (Trigger.isInsert ? 'Insert' : 'Update'), n, o);
    }
}
