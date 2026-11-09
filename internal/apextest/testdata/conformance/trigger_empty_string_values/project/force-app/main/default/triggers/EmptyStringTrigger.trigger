trigger EmptyStringTrigger on Account (before insert, before update, after insert, after update) {
    for (Account row : Trigger.new) {
        if (row.Name == 'EmptyTrigger Probe') {
            EmptyTriggerProbe.record((Trigger.isBefore ? 'before' : 'after') + (Trigger.isInsert ? 'Insert' : 'Update') + (Trigger.isUpdate && row.Description == null ? 'Null' : (Trigger.isUpdate && row.Description == '' ? 'Empty' : '')), row);
            if (Trigger.isUpdate) {
                Account previous = Trigger.oldMap.get(row.Id);
                EmptyTriggerProbe.record((Trigger.isBefore ? 'before' : 'after') + 'UpdateOld' + (previous.Description == null ? 'Null' : 'Value'), previous);
            }
        }
    }
}
