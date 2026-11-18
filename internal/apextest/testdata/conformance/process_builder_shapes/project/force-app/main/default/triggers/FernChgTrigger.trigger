trigger FernChgTrigger on Fern_Chg__c (before insert, after insert, before update, after update) { FernLog.fire('Chg'); }
