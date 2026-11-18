trigger FernFlowTrigger on Fern_Flow__c (before insert, after insert, before update, after update) { FernLog.fire('Flow'); }
