trigger CdcSubscriber on AccountChangeEvent (after insert) {
 List<EventLog__c> rows=new List<EventLog__c>();
 for (AccountChangeEvent item: Trigger.new) {
  EventBus.ChangeEventHeader h=item.ChangeEventHeader;
  rows.add(new EventLog__c(Name='CDC-'+h.changeType, Kind__c=h.entityName));
 }
 insert rows;
}
