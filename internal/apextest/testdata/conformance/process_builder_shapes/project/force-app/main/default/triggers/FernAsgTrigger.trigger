trigger FernAsgTrigger on Fern_Asg__c (before insert, after insert, before update, after update) { FernLog.fire('Asg'); }
