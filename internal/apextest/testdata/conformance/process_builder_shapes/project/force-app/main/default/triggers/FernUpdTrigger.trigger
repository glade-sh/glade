trigger FernUpdTrigger on Fern_Upd__c (before insert, after insert, before update, after update) { FernLog.fire('Upd'); }
