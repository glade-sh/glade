trigger FernSibTrigger on Fern_Sib__c (before insert, after insert, before update, after update) { FernLog.fire('Sib'); }
