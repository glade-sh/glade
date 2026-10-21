trigger C155NIObserve on C155Number__c (before insert,after insert){for(C155Number__c w:Trigger.new){C155NITriggerView.observe(w,Trigger.isBefore?'before':'after');}}
