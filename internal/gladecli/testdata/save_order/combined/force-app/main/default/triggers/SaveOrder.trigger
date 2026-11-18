trigger SaveOrder on Account (before insert, after insert) {
 if (Trigger.isBefore) for (Account a : Trigger.new) { a.Description += 'B'; SaveOrderTrace.events += 'before:' + a.Name + ':' + a.Description + ';'; }
 if (Trigger.isAfter) for (Account a : Trigger.new) { SaveOrderTrace.events += 'after:' + a.Name + ':' + a.Description + ';'; if (a.Name == 'reject-after') a.addError('probe after reject'); }
}
