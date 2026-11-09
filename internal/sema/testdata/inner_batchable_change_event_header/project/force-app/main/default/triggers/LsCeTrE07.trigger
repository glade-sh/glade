trigger LsCeTrE07 on AccountChangeEvent (after insert) {
 for (AccountChangeEvent ev : Trigger.new) {
  EventBus.ChangeEventHeader h = ev.ChangeEventHeader;
  Object v = h.getCommitUser();
  System.debug(v);
 }
}
