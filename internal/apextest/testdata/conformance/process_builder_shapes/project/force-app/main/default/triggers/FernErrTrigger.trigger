trigger FernErrTrigger on Fern_Err__c (before insert, after insert, before update, after update) { FernLog.fire('Err'); }
