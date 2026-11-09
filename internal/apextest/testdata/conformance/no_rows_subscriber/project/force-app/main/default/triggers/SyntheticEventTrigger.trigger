trigger SyntheticEventTrigger on SyntheticEvent__e (after insert) { for (SyntheticEvent__e item : Trigger.new) { System.debug(item); } }
