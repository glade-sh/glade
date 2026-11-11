trigger FernMet62Trigger on Fern_Met62__c (before insert, after insert, before update, after update) { FernLog.fire('Met62'); }
