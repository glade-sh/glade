trigger LsCeTrE15 on AccountChangeEvent (after insert) {
 for (AccountChangeEvent ev : Trigger.new) {
  EventBus.ChangeEventHeader h = ev.ChangeEventHeader;
  Object v = ev.ChangeEventHeader.recordIds;
  System.debug(v);
 }
}
