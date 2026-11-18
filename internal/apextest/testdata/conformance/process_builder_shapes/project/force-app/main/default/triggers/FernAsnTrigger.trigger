trigger FernAsnTrigger on Fern_Asn__c (before insert, after insert, before update, after update) { FernLog.fire('Asn'); }
