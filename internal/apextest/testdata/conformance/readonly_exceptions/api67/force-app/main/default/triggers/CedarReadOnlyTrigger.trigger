trigger CedarReadOnlyTrigger on Contact (after insert, after update, before delete, after delete) {
    if (CedarReadOnlyProbe.enabled) {
        String phase = (Trigger.isBefore ? 'before' : 'after') + (Trigger.isInsert ? 'Insert' : (Trigger.isUpdate ? 'Update' : 'Delete'));
        if (phase == CedarReadOnlyProbe.phase) {
            Contact record = Trigger.isDelete ? Trigger.old[0] : Trigger.new[0];
            CedarReadOnlyProbe.observe(record, Trigger.new);
        }
    }
}
