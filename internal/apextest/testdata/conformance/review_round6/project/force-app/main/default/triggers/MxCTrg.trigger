trigger MxCTrg on MxC__c (after insert, after update) {
MxP__c p = [SELECT Cnt__c, Sm__c, Mx__c FROM MxP__c WHERE Id = :Trigger.new[0].P__c];
MxLog.seen += (Trigger.isInsert ? 'CI=' : 'CU=') + MxLog.fmt(p) + ';';
}
