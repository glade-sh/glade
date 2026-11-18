RtParent__c p = new RtParent__c(Name = 'r11');
insert p;
insert new List<RtChild__c>{
    new RtChild__c(Name = 'c1', Amount__c = 1, Included__c = true, Parent__c = p.Id),
    new RtChild__c(Name = 'c2', Amount__c = 2, Included__c = true, Parent__c = p.Id),
    new RtChild__c(Name = 'c3', Amount__c = 3, Included__c = true, Parent__c = p.Id),
    new RtChild__c(Name = 'c4', Amount__c = 4, Included__c = true, Parent__c = p.Id)};
List<Id> ids = new List<Id>();
for (RtChild__c c : [SELECT Id FROM RtChild__c WHERE Parent__c = :p.Id]) {
    ids.add(c.Id);
}
RtLog.reset();
List<Database.DeleteResult> rs = Database.delete(ids, false);
System.debug('OBS|delevdb|n=' + RtLog.delCount + '|sz=' + String.join(RtLog.delSizes, ',') + '|ok=' + rs.size());
