trigger DpATrg on DpA__c (after update) {
    DpCtl.depth++;
    Integer d = DpCtl.depth;
    List<DpB__c> bs = [SELECT Id FROM DpB__c LIMIT 1];
    DpB__c b = new DpB__c(Id = bs[0].Id, Name = 'b' + d);
    if (DpCtl.mode == 'throw') {
        update b;
    } else if (DpCtl.mode == 'catchlog') {
        try { update b; } catch (Exception e) { DpCtl.add(d + DpCtl.type(e)); throw e; }
    } else {
        try { update b; } catch (Exception e) { DpCtl.add(d + DpCtl.type(e) + 'S'); }
    }
}
