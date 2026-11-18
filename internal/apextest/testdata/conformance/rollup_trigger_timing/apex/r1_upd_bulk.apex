RtParent__c p = new RtParent__c(Name = 'r4');
insert p;
List<RtChild__c> cs = new List<RtChild__c>{
    new RtChild__c(Name = 'c1', Amount__c = 10, Included__c = true, Parent__c = p.Id),
    new RtChild__c(Name = 'c2', Amount__c = 5, Included__c = false, Parent__c = p.Id)};
insert cs;
RtLog.reset();
cs[0].Amount__c = 20;
cs[0].Included__c = true;
cs[1].Amount__c = 7;
cs[1].Included__c = true;
update cs;
String t = RtLog.take();
RtParent__c q = [SELECT Count__c, SumAmt__c, MinAmt__c, MaxAmt__c, FiltSum__c, FiltCount__c FROM RtParent__c WHERE Id = :p.Id];
System.debug('OBS|upd2|T=' + t + '|P=' + RtFmt.of(q));
