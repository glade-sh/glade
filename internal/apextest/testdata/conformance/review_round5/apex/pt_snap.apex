PtP__c p = new PtP__c(Name='p'); insert p; PtC__c c1 = new PtC__c(Name='a', P__c=p.Id, Amt__c=1); insert c1;
PtC__c q = [SELECT Id, P__r.Cnt__c FROM PtC__c WHERE Id = :c1.Id];
insert new PtC__c(Name='b', P__c=p.Id, Amt__c=1);
PtC__c q2 = [SELECT Id, P__r.Cnt__c FROM PtC__c WHERE Id = :c1.Id];
System.debug('OBS|pt_snap|before=' + q.P__r.Cnt__c + '|after=' + q2.P__r.Cnt__c);