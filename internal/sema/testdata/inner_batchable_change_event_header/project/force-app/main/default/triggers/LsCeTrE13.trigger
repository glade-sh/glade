trigger LsCeTrE13 on AccountChangeEvent (after insert) {
 for (AccountChangeEvent ev : Trigger.new) {
  EventBus.ChangeEventHeader h = ev.ChangeEventHeader;
  Object v = ev.ChangeEventHeader.entityName;
  System.debug(v);
 }
}
