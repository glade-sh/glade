trigger FernPrtTrigger on Fern_Prt__c (before insert, after insert, before update, after update) { FernLog.fire('Prt'); }
