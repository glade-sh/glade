trigger FernEmlTrigger on Fern_Eml__c (before insert, after insert, before update, after update) { FernLog.fire('Eml'); }
