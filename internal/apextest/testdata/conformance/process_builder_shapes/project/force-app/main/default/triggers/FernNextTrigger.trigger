trigger FernNextTrigger on Fern_Next__c (before insert, after insert, before update, after update) { FernLog.fire('Next'); }
