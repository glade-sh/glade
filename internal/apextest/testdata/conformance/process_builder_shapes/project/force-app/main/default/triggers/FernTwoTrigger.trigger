trigger FernTwoTrigger on Fern_Two__c (before insert, after insert, before update, after update) { FernLog.fire('Two'); }
