trigger LsCeTrE09 on AccountChangeEvent (after insert) {
 for (AccountChangeEvent ev : Trigger.new) {
  EventBus.ChangeEventHeader h = ev.ChangeEventHeader;
  Object v = ev.ChangeEventHeader.getEntityName();
  System.debug(v);
 }
}
