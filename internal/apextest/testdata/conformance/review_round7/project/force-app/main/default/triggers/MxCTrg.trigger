trigger MxCTrg on MxC__c (after insert, after update) {
MxP__c p = [SELECT Name, Cnt__c, Sm__c, Mx__c, Mn__c FROM MxP__c WHERE Id = :Trigger.new[0].P__c];
MxLog.seen += (Trigger.isInsert ? 'CI' : 'CU') + p.Name + '=' + MxLog.fmt(p) + ';';
}