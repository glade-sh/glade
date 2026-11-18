NsP__c p = new NsP__c(Name='np'); insert p;
insert new NsC__c(Name='old', P__c=p.Id, Amt__c=5);
NsCtl.reset('sib', 3, new List<Integer>{30,40,50});
insert new NsC__c(Name='l1', P__c=p.Id, Amt__c=30);
NsP__c q = [SELECT Cnt__c, Sm__c, Mn__c, Mx__c, Fc__c FROM NsP__c WHERE Id = :p.Id];
System.debug('OBS|n_sib3up|' + NsCtl.log + 'final=' + NsCtl.fmt(q));