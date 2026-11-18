trigger MxPTrg on MxP__c (after update) {
MxP__c p = [SELECT Cnt__c, Sm__c, Mx__c FROM MxP__c WHERE Id = :Trigger.new[0].Id];
MxLog.seen += 'P=' + MxLog.fmt(p) + ';';
if (MxLog.mode == 'nested' && !MxLog.fired) { MxLog.fired = true; insert new MxC__c(Name = 'nest', P__c = p.Id, Amt__c = 1, ExtId__c = 'nest' + String.valueOf(Crypto.getRandomInteger())); }
}
