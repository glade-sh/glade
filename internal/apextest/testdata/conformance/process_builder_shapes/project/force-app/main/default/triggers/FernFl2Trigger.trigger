trigger FernFl2Trigger on Fern_Fl2__c (before insert, after insert, before update, after update) { FernLog.fire('Fl2'); }
