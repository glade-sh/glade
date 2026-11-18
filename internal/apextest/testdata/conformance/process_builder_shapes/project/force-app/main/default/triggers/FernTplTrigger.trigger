trigger FernTplTrigger on Fern_Tpl__c (before insert, after insert, before update, after update) { FernLog.fire('Tpl'); }
