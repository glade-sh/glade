trigger LsCeTrE04 on AccountChangeEvent (after insert) {
 for (AccountChangeEvent ev : Trigger.new) {
  EventBus.ChangeEventHeader h = ev.ChangeEventHeader;
  Object v = h.getChangeType();
  System.debug(v);
 }
}
