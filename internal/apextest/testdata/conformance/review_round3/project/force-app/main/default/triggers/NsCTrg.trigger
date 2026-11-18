trigger NsCTrg on NsC__c (after insert) {
  NsCtl.level++; Integer l = NsCtl.level; Id pid = Trigger.new[0].P__c;
  if (NsCtl.mode == 'sib' && l < NsCtl.maxLevel) { insert new NsC__c(Name = 'l' + (l + 1), P__c = pid, Amt__c = NsCtl.amts[l]); }
  if (NsCtl.mode == 'del' && l == 1) { delete [SELECT Id FROM NsC__c WHERE Name = 'old']; }
  NsP__c p = [SELECT Cnt__c, Sm__c, Mn__c, Mx__c, Fc__c FROM NsP__c WHERE Id = :pid];
  NsCtl.log += 'L' + l + '=' + NsCtl.fmt(p) + ';';
}
