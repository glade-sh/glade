trigger FernGpkTrigger on Fern_Gpk__c (before insert, after insert, before update, after update) { FernLog.fire('Gpk'); }
