MxP__c pA = new MxP__c(Name='A'); insert pA;
insert new MxC__c(Name='ao', P__c=pA.Id, Amt__c=10, ExtId__c='ao');
MxP__c pB = new MxP__c(Name='B'); insert pB;
insert new MxC__c(Name='bo', P__c=pB.Id, Amt__c=10, ExtId__c='bo');
MxLog.reset('none');
upsert new List<MxC__c>{new MxC__c(ExtId__c='bo', Name='bo2', P__c=pB.Id, Amt__c=20),new MxC__c(ExtId__c='an', Name='an2', P__c=pA.Id, Amt__c=7)} MxC__c.ExtId__c;
System.debug('OBS|b_two|' + MxLog.seen + 'FA=' + MxLog.fmt([SELECT Cnt__c, Sm__c, Mx__c, Mn__c FROM MxP__c WHERE Id = :pA.Id]) + ';' + 'FB=' + MxLog.fmt([SELECT Cnt__c, Sm__c, Mx__c, Mn__c FROM MxP__c WHERE Id = :pB.Id]) + ';');