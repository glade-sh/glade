trigger ReviewRouteObserver on Account (before insert, before update) { for (Account a : Trigger.new) ReviewRouteTrace.kinds.add(a.IsPersonAccount); }
