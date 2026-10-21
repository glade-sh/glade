@IsTest private class TriggerFailureObservation {
 private static String clean(String message, List<Account> rows) {
  for (Account row : rows) if (row.Id != null) message = message.replace(String.valueOf(row.Id), '<id>');
  return message;
 }
 private static List<Object> errors(Database.Error[] failures, List<Account> rows) {
  List<Object> result = new List<Object>();
  for (Database.Error failure : failures) result.add(new Map<String,Object>{'status'=>String.valueOf(failure.getStatusCode()), 'message'=>clean(failure.getMessage(), rows), 'fields'=>failure.getFields()});
  return result;
 }
 private static void observe(String phase, String fault) {
  List<Object> result = new List<Object>();
  for (String mode : new List<String>{'partial','atomic','statement'}) {
   TriggerFailureControl.phase = null;
   List<Account> rows = new List<Account>{new Account(Name='Clean'),new Account(Name='Fault')};
   String operation = phase.endsWith('insert') ? 'insert' : (phase.endsWith('update') ? 'update' : 'delete');
   if (operation != 'insert') insert rows;
   for (Account row : rows) row.Description = 'changed';
   TriggerFailureControl.phase = phase;
   TriggerFailureControl.fault = fault;
   Map<String,Object> observation = new Map<String,Object>{'mode'=>mode};
   try {
    if (mode == 'statement') {
     if (operation == 'insert') insert rows;
     else if (operation == 'update') update rows;
     else delete rows;
     observation.put('outcome','success');
    } else if (operation == 'delete') {
     List<Object> saved = new List<Object>();
     for (Database.DeleteResult item : Database.delete(rows, mode == 'atomic')) saved.add(new Map<String,Object>{'success'=>item.isSuccess(),'errors'=>errors(item.getErrors(),rows)});
     observation.put('results',saved);
    } else {
     Database.SaveResult[] saved;
     if (operation == 'insert') saved = Database.insert(rows, mode == 'atomic');
     else saved = Database.update(rows, mode == 'atomic');
     List<Object> output = new List<Object>();
     for (Database.SaveResult item : saved) output.add(new Map<String,Object>{'success'=>item.isSuccess(),'errors'=>errors(item.getErrors(),rows)});
     observation.put('results',output);
    }
   } catch (DmlException failure) {
    observation.put('outcome',failure.getTypeName());
    observation.put('message',clean(failure.getMessage(),rows));
    List<Object> details = new List<Object>();
    for (Integer i=0;i<failure.getNumDml();i++) details.add(new Map<String,Object>{'index'=>failure.getDmlIndex(i),'status'=>String.valueOf(failure.getDmlStatusCode(i)),'message'=>clean(failure.getDmlMessage(i),rows),'fields'=>failure.getDmlFieldNames(i)});
    observation.put('details',details);
   }
   observation.put('stored',[SELECT COUNT() FROM Account]);
   Integer changed = 0;
   for (Account stored : [SELECT Description FROM Account]) if (stored.Description == 'changed') changed++;
   observation.put('changed',changed);
   observation.put('sideEffects',[SELECT COUNT() FROM Contact]);
   result.add(observation);
   TriggerFailureControl.phase = null;
   delete [SELECT Id FROM Account];
   delete [SELECT Id FROM Contact];
  }
  System.assert(false,'OBSERVED|'+JSON.serialize(result));
 }
 @IsTest static void beforeinsert_throw() { observe('beforeinsert','throw'); }
 @IsTest static void beforeinsert_null() { observe('beforeinsert','null'); }
 @IsTest static void beforeinsert_row() { observe('beforeinsert','row'); }
 @IsTest static void beforeinsert_field() { observe('beforeinsert','field'); }
 @IsTest static void afterinsert_throw() { observe('afterinsert','throw'); }
 @IsTest static void afterinsert_null() { observe('afterinsert','null'); }
 @IsTest static void afterinsert_row() { observe('afterinsert','row'); }
 @IsTest static void afterinsert_field() { observe('afterinsert','field'); }
 @IsTest static void beforeupdate_throw() { observe('beforeupdate','throw'); }
 @IsTest static void beforeupdate_null() { observe('beforeupdate','null'); }
 @IsTest static void beforeupdate_row() { observe('beforeupdate','row'); }
 @IsTest static void beforeupdate_field() { observe('beforeupdate','field'); }
 @IsTest static void afterupdate_throw() { observe('afterupdate','throw'); }
 @IsTest static void afterupdate_null() { observe('afterupdate','null'); }
 @IsTest static void afterupdate_row() { observe('afterupdate','row'); }
 @IsTest static void afterupdate_field() { observe('afterupdate','field'); }
 @IsTest static void beforedelete_throw() { observe('beforedelete','throw'); }
 @IsTest static void beforedelete_null() { observe('beforedelete','null'); }
 @IsTest static void beforedelete_row() { observe('beforedelete','row'); }
 @IsTest static void beforedelete_field() { observe('beforedelete','field'); }
 @IsTest static void afterdelete_throw() { observe('afterdelete','throw'); }
 @IsTest static void afterdelete_null() { observe('afterdelete','null'); }
 @IsTest static void afterdelete_row() { observe('afterdelete','row'); }
 @IsTest static void afterdelete_field() { observe('afterdelete','field'); }
}
