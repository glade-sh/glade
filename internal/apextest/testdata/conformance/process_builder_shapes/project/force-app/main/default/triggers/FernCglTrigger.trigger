trigger FernCglTrigger on Fern_Cgl__c (before insert, after insert, before update, after update) { FernLog.fire('Cgl'); }
