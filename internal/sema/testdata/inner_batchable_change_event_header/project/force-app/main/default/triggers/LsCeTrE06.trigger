trigger LsCeTrE06 on AccountChangeEvent (after insert) {
 for (AccountChangeEvent ev : Trigger.new) {
  EventBus.ChangeEventHeader h = ev.ChangeEventHeader;
  Object v = h.getRecordIds();
  System.debug(v);
 }
}
