trigger FernOrdTrigger on Fern_Ord__c (before insert, after insert, before update, after update) { FernLog.fire('Ord'); }
