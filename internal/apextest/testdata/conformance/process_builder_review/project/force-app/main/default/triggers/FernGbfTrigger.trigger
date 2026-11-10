trigger FernGbfTrigger on Fern_Gbf__c (before insert, after insert, before update, after update) { FernLog.fire('Gbf'); }
