trigger LsCeTrE03 on AccountChangeEvent (after insert) {
 for (AccountChangeEvent ev : Trigger.new) {
  EventBus.ChangeEventHeader h = ev.ChangeEventHeader;
  Object v = h.changeType;
  System.debug(v);
 }
}
