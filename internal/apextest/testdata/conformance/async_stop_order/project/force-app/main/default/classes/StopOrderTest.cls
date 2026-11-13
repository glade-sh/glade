@IsTest
private class StopOrderTest {
    private static void finish(Boolean cron) {
        Test.stopTest();
        System.assert(false, StopOrderMk.observe(cron));
    }
    private static void q(String n) {
        System.enqueueJob(new StopOrderQ(n, false));
    }
    @IsTest static void qqf() {
        Test.startTest(); q('k'); q('t'); StopOrderMk.fut('b'); finish(false);
    }
    @IsTest static void fqq() {
        Test.startTest(); StopOrderMk.fut('b'); q('k'); q('t'); finish(false);
    }
    @IsTest static void qfqf() {
        Test.startTest(); q('k'); StopOrderMk.fut('b'); q('t'); StopOrderMk.fut('e'); finish(false);
    }
    @IsTest static void ff() {
        Test.startTest(); StopOrderMk.fut('b'); StopOrderMk.fut('e'); finish(false);
    }
    @IsTest static void bqf() {
        Test.startTest(); Database.executeBatch(new StopOrderBatch()); q('k'); StopOrderMk.fut('b'); finish(false);
    }
    @IsTest static void gen2f() {
        Test.startTest(); System.enqueueJob(new StopOrderQ('k', true)); q('t'); finish(false);
    }
    @IsTest static void gen2q() {
        Test.startTest(); StopOrderMk.futEnqueue('b', 'n'); q('k'); finish(false);
    }
    @IsTest static void trig() {
        Test.startTest(); insert new Contact(LastName = 'x'); StopOrderMk.fut('b'); finish(false);
    }
    @IsTest static void pre() {
        q('c'); Test.startTest(); StopOrderMk.fut('b'); q('k'); finish(false);
    }
    @IsTest static void qqfRepeat() {
        Test.startTest(); q('k'); q('t'); StopOrderMk.fut('b'); finish(false);
    }
    @IsTest static void sched() {
        Test.startTest(); System.schedule('stopOrderJob', '0 0 0 1 1 ? 2099', new StopOrderSched()); finish(true);
    }
    @IsTest static void qfs() {
        Test.startTest(); q('k'); StopOrderMk.fut('b'); System.schedule('stopOrderJob', '0 0 0 1 1 ? 2099', new StopOrderSched()); finish(true);
    }
}
