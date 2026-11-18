trigger CheckboxCapture on Item__c (after insert, after update) {
 for (Item__c row : Trigger.new) CheckboxObserver.save(Trigger.isInsert ? 'insert' : 'update',row);
}
