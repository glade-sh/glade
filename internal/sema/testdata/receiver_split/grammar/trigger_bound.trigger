trigger RouteTrig on Account (before insert) { if (true) { System.debug(Trigger.
old); } }
