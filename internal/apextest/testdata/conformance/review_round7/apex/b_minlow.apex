MxP__c pA = new MxP__c(Name='A'); insert pA;
insert new MxC__c(Name='o', P__c=pA.Id, Amt__c=10, ExtId__c='o');
MxLog.reset('none');
upsert new List<MxC__c>{new MxC__c(ExtId__c='o', Name='o2', P__c=pA.Id, Amt__c=20),new MxC__c(ExtId__c='n', Name='n2', P__c=pA.Id, Amt__c=3)} MxC__c.ExtId__c;
System.debug('OBS|b_minlow|' + MxLog.seen + 'FA=' + MxLog.fmt([SELECT Cnt__c, Sm__c, Mx__c, Mn__c FROM MxP__c WHERE Id = :pA.Id]) + ';');