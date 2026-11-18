trigger FernAsuTrigger on Fern_Asu__c (before insert, after insert, before update, after update) { FernLog.fire('Asu'); }
