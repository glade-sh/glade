trigger FernFgdTrigger on Fern_Fgd__c (before insert, after insert, before update, after update) { FernLog.fire('Fgd'); }
