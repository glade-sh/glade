trigger FernGbiTrigger on Fern_Gbi__c (before insert, after insert, before update, after update) { FernLog.fire('Gbi'); }
