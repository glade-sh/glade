trigger Review349Observer on Account (before insert, before update) { for (Account a : Trigger.new) Review349Trace.beforeKinds.add(a.IsPersonAccount); }
