trigger SecondEventTrigger on SecondEvent__e (after insert) { for (SecondEvent__e item : Trigger.new) { System.debug(item); } }
