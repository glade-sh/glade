trigger FernBmpTrigger on Fern_Bmp__c (before insert, after insert, before update, after update) { FernLog.fire('Bmp'); }
