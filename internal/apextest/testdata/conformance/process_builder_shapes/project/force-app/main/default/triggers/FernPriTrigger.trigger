trigger FernPriTrigger on Fern_Pri__c (before insert, after insert, before update, after update) { FernLog.fire('Pri'); }
