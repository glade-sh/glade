trigger EmailBoundaryObserver on Account (before insert, before update) {
 for(Account a:Trigger.new) EmailBoundaryTrace.events.add(String.valueOf(a.IsPersonAccount));
}
