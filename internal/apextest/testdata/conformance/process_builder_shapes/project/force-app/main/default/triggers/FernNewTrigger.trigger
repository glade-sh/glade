trigger FernNewTrigger on Fern_New__c (before insert, after insert, before update, after update) { FernLog.fire('New'); }
