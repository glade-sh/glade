trigger FernMet67Trigger on Fern_Met67__c (before insert, after insert, before update, after update) { FernLog.fire('Met67'); }
