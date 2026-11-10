trigger FernRecTrigger on Fern_Rec__c (before insert, after insert, before update, after update) { FernLog.fire('Rec'); }
