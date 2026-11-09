trigger LsCeTrE08 on AccountChangeEvent (after insert) {
 for (AccountChangeEvent ev : Trigger.new) {
  EventBus.ChangeEventHeader h = ev.ChangeEventHeader;
  Object v = h.getChangedFields();
  System.debug(v);
 }
}
