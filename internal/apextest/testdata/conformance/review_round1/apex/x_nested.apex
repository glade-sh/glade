HiLog.nested = false; HiLog.seen = '';
HiP__c p = new HiP__c(Name='np'); insert p;
insert new HiC__c(Name='a', P__c=p.Id);
System.debug('OBS|nested|' + HiLog.seen + '|final=' + [SELECT Cnt__c FROM HiP__c WHERE Id = :p.Id].Cnt__c);