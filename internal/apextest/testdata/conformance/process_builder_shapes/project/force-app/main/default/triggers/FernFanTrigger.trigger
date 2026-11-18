trigger FernFanTrigger on Fern_Fan__c (before insert, after insert, before update, after update) { FernLog.fire('Fan'); }
