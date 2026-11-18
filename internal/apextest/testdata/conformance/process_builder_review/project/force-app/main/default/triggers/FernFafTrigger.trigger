trigger FernFafTrigger on Fern_Faf__c (before insert, after insert, before update, after update) { FernLog.fire('Faf'); }
