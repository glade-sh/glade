RtParent__c p = new RtParent__c(Name = 'r10');
insert p;
RtChild__c c = new RtChild__c(Name = 'c1', Amount__c = 1, Included__c = true, Parent__c = p.Id);
insert c;
RtLog.reset();
delete c;
System.debug('OBS|delev1|n=' + RtLog.delCount + '|sz=' + String.join(RtLog.delSizes, ',') + '|js=' + JSON.serialize(RtLog.delSizes));
