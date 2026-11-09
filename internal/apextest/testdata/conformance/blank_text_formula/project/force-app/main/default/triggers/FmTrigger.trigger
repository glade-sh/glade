trigger FmTrigger on Fm__c (before insert, after insert, before update, after update) {
    for (Fm__c n : Trigger.new) {
        if (n.Name != 'FmProbeRow') continue;
        Fm__c o = Trigger.isUpdate ? Trigger.oldMap.get(n.Id) : null;
        FmProbe.record((Trigger.isBefore ? 'before' : 'after') + (Trigger.isInsert ? 'Insert' : 'Update'), n, o);
    }
}
