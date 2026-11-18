
RtLog.obs = 'S';
RtParent__c p = new RtParent__c(Name = 'r13');
insert p;
insert new RtChild__c(Name = 'c1', Amount__c = 1, Included__c = true, Parent__c = p.Id);
System.debug('OBS|statics|after=' + RtLog.obs);
