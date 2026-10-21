trigger OwnedFlowTrigger on Account (before insert, after insert, before update, after update, before delete, after delete) {
    String stage = Trigger.isBefore ? 'before' : 'after';
    if (Trigger.isInsert) stage += 'Insert';
    if (Trigger.isUpdate) stage += 'Update';
    if (Trigger.isDelete) stage += 'Delete';
    if (OwnedFlowHandler.stage == stage) {
        OwnedFlowHandler.handle(Trigger.isDelete ? Trigger.old : Trigger.new,
                               Trigger.isBefore && !Trigger.isDelete);
    }
}
