trigger FernRelTrigger on Fern_Rel__c (before insert, after insert, before update, after update) { FernLog.fire('Rel'); }
