trigger FernGroupTrigger on Fern_Group__c (before insert, after insert, before update, after update) { FernLog.fire('Group'); }
