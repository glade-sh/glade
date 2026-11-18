trigger FernThrTrigger on Fern_Thr__c (before insert, after insert, before update, after update) { FernLog.fire('Thr'); }
