trigger FernFmaTrigger on Fern_Fma__c (before insert, after insert, before update, after update) { FernLog.fire('Fma'); }
