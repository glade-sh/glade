trigger FernRgdTrigger on Fern_Rgd__c (before insert, after insert, before update, after update) { FernLog.fire('Rgd'); }
