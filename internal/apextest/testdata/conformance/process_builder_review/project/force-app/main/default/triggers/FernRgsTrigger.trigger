trigger FernRgsTrigger on Fern_Rgs__c (before insert, after insert, before update, after update) { FernLog.fire('Rgs'); }
