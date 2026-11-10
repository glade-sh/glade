trigger FernRootTrigger on Fern_Root__c (before insert, after insert, before update, after update) { FernLog.fire('Root'); }
