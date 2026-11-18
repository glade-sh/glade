@IsTest
private class DpRouteTest {
    @IsTest
    static void depthMessage() {
        insert new DpA__c(Name = 'A1');
        insert new DpB__c(Name = 'B1');
        DpCtl.reset('throw');
        DpA__c a = [SELECT Id FROM DpA__c LIMIT 1];
        a.Name = 't0';
        try {
            update a;
            System.assert(false, 'ok|d=' + DpCtl.depth);
        } catch (Exception e) {
            System.assert(false, 'd=' + DpCtl.depth + '|' + DpCtl.loc(e));
        }
    }
}
