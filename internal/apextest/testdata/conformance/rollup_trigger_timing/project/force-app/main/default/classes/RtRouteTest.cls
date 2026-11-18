@IsTest
private class RtRouteTest {
    @IsTest
    static void insFull() {
        RtParent__c p = new RtParent__c(Name = 't1');
        insert p;
        insert new RtChild__c(Name = 'c1', Amount__c = 10, Included__c = true, Parent__c = p.Id);
        System.assert(false, 'T=' + RtLog.obs + '|P=' + post(p.Id));
    }
    @IsTest
    static void updFull() {
        RtParent__c p = new RtParent__c(Name = 't2');
        insert p;
        RtChild__c c = new RtChild__c(Name = 'c1', Amount__c = 10, Included__c = true, Parent__c = p.Id);
        insert c;
        RtLog.reset();
        c.Amount__c = 30;
        update c;
        System.assert(false, 'T=' + RtLog.obs + '|P=' + post(p.Id));
    }
    @IsTest
    static void delFull() {
        RtParent__c p = new RtParent__c(Name = 't3');
        insert p;
        List<RtChild__c> cs = new List<RtChild__c>{
            new RtChild__c(Name = 'c1', Amount__c = 10, Included__c = true, Parent__c = p.Id),
            new RtChild__c(Name = 'c2', Amount__c = 5, Included__c = true, Parent__c = p.Id)};
        insert cs;
        RtLog.reset();
        delete cs[0];
        System.assert(false, 'T=' + RtLog.obs + '|P=' + post(p.Id));
    }
    @IsTest
    static void undFull() {
        RtParent__c p = new RtParent__c(Name = 't4');
        insert p;
        List<RtChild__c> cs = new List<RtChild__c>{
            new RtChild__c(Name = 'c1', Amount__c = 10, Included__c = true, Parent__c = p.Id),
            new RtChild__c(Name = 'c2', Amount__c = 5, Included__c = true, Parent__c = p.Id)};
        insert cs;
        delete cs[0];
        RtLog.reset();
        undelete cs[0];
        System.assert(false, 'T=' + RtLog.obs + '|P=' + post(p.Id));
    }
    static String post(Id pid) {
        RtParent__c q = [SELECT Count__c, SumAmt__c, MinAmt__c, MaxAmt__c, FiltSum__c, FiltCount__c FROM RtParent__c WHERE Id = :pid];
        return RtFmt.of(q);
    }
}
