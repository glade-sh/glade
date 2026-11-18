trigger FernFmlTrigger on Fern_Fml__c (before insert, after insert, before update, after update) { FernLog.fire('Fml'); }
