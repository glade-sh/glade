trigger FernPaTrigger on Fern_Pa__c (before insert, after insert, before update, after update) { FernLog.fire('Pa'); }
