trigger DpBTrg on DpB__c (after update) {
    // alternate arm of the ping-pong; DML sits on a different line than DpATrg
    DpCtl.depth++;
    Integer d = DpCtl.depth;
    List<DpA__c> xs = [SELECT Id FROM DpA__c LIMIT 1];
    DpA__c a = new DpA__c(Id = xs[0].Id, Name = 'a' + d);
    if (DpCtl.mode == 'throw') {
        update a;
    } else if (DpCtl.mode == 'catchlog') {
        try { update a; } catch (Exception e) {
            if (d == 16) { DpCtl.counters = 'dml' + Limits.getDmlStatements() + ',rows' + Limits.getDmlRows() + ',q' + Limits.getQueries() + ',qr' + Limits.getQueryRows(); }
            DpCtl.add(d + DpCtl.type(e));
            throw e;
        }
    } else {
        try { update a; } catch (Exception e) { DpCtl.add(d + DpCtl.type(e) + 'S'); }
    }
}
