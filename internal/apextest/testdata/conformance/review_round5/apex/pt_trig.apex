PtLog.reset(); PtP__c p = new PtP__c(Name='p'); insert p; insert new PtC__c(Name='a', P__c=p.Id, Amt__c=1);
System.debug('OBS|pt_trig|' + PtLog.seen);