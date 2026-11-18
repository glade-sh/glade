trigger MxPTrg on MxP__c (after update) {
for (MxP__c n : Trigger.new) {
MxP__c o = Trigger.oldMap.get(n.Id);
MxP__c q = [SELECT Name, Cnt__c, Sm__c, Mx__c, Mn__c FROM MxP__c WHERE Id = :n.Id];
MxLog.seen += 'P' + n.Name + '=new:' + MxLog.fmt(n) + ',old:' + MxLog.fmt(o) + ',q:' + MxLog.fmt(q) + ';';
}
}