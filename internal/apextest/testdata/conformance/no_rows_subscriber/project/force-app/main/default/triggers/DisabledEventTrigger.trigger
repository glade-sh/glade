trigger DisabledEventTrigger on DisabledEvent__e (after insert) { for (DisabledEvent__e item : Trigger.new) { System.debug(item); } }
