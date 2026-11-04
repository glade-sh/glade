trigger ExternalObserver on Account (before insert, before update) { for(Account a: Trigger.new) ExternalTrace.beforeValues.add(a.IsPersonAccount); }
