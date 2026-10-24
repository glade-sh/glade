trigger CedarChangeObserver on AccountChangeEvent (after insert) {
 List<Contact> output=new List<Contact>();
 for(AccountChangeEvent eventRow:Trigger.new){
  EventBus.ChangeEventHeader h=eventRow.ChangeEventHeader;
  output.add(new Contact(LastName='cedar-cdc-'+h.changeType, Description=h.entityName+':'+h.recordIds.size()+':'+eventRow.Name));
 }
 insert output;
}
