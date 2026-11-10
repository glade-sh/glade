trigger FernGblTrigger on Fern_Gbl__c (before insert, after insert, before update, after update) { FernLog.fire('Gbl'); }
