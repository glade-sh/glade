trigger AccountFailureProbe on Account (before insert, after insert, before update, after update, before delete, after delete) {
 String phase = (Trigger.isBefore ? 'before' : 'after') + (Trigger.isInsert ? 'insert' : (Trigger.isUpdate ? 'update' : 'delete'));
 if (TriggerFailureControl.phase == phase) {
  List<Account> rows = Trigger.isDelete ? Trigger.old : Trigger.new;
  for (Account row : rows) {
   if (row.Name == 'Fault') {
    if (TriggerFailureControl.fault == 'throw') {
     insert new Contact(LastName = 'Trigger side effect');
     throw new TriggerFailureControl.SyntheticException('synthetic trigger fault');
    } else if (TriggerFailureControl.fault == 'null') {
     String missing;
     missing.toLowerCase();
    } else if (TriggerFailureControl.fault == 'field') {
     row.Name.addError('synthetic field error');
    } else {
     row.addError('synthetic row error');
    }
   }
  }
 }
}
