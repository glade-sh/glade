RtLog.reset();
RtParent__c p = new RtParent__c(Name = 'r2');
insert p;
insert new List<RtChild__c>{
    new RtChild__c(Name = 'c1', Amount__c = 10, Included__c = true, Parent__c = p.Id),
    new RtChild__c(Name = 'c2', Amount__c = 5, Included__c = false, Parent__c = p.Id)};
String t = RtLog.take();
RtParent__c q = [SELECT Count__c, SumAmt__c, MinAmt__c, MaxAmt__c, FiltSum__c, FiltCount__c FROM RtParent__c WHERE Id = :p.Id];
System.debug('OBS|ins2|T=' + t + '|P=' + RtFmt.of(q));
