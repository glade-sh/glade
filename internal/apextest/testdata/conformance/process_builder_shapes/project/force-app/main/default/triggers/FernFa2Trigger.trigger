trigger FernFa2Trigger on Fern_Fa2__c (before insert, after insert, before update, after update) { FernLog.fire('Fa2'); }
