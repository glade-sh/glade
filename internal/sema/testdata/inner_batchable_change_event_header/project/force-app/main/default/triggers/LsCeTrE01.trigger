trigger LsCeTrE01 on AccountChangeEvent (after insert) {
 for (AccountChangeEvent ev : Trigger.new) {
  EventBus.ChangeEventHeader h = ev.ChangeEventHeader;
  Object v = ev.ChangeEventHeader.changeType;
  System.debug(v);
 }
}
