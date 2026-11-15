@IsTest
private class StopOrderMixTest {
    private static String sub = 'ok';
    private static void fin() {
        String stop = 'ok';
        try {
            Test.stopTest();
        } catch (Exception e) {
            stop = e.getTypeName() + ':' + e.getMessage();
        }
        System.assert(false, 'sub=' + sub + '|stop=' + stop + '|' + StopOrderMk.observe(false).substringAfter('stop=ok|'));
    }
    private static void q(String n) {
        System.enqueueJob(new StopOrderQ(n, false));
    }
    private static void b(String p) {
        Database.executeBatch(new StopOrderNB(p, false, false, false));
    }
    private static void f(String n) {
        StopOrderMk.fut(n);
    }
    private static void fail(Exception e) {
        sub = e.getTypeName() + ':' + e.getMessage();
    }
    @IsTest static void qb() {
        Test.startTest(); try { q('k'); b('a'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void qbf() {
        Test.startTest(); try { q('k'); b('a'); f('f'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void qfb() {
        Test.startTest(); try { q('k'); f('f'); b('a'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void fb() {
        Test.startTest(); try { f('f'); b('a'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bf() {
        Test.startTest(); try { b('a'); f('f'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bonly() {
        Test.startTest(); try { b('a'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bb() {
        Test.startTest(); try { b('a'); b('c'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bqb() {
        Test.startTest(); try { b('a'); q('k'); b('c'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bsq() {
        Test.startTest(); try { Database.executeBatch(new StopOrderNB('a', true, false, false)); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bsqk() {
        Test.startTest(); try { Database.executeBatch(new StopOrderNB('a', true, false, false)); q('k'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bxq() {
        Test.startTest(); try { Database.executeBatch(new StopOrderNB('a', false, true, false)); q('k'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bfq() {
        Test.startTest(); try { Database.executeBatch(new StopOrderNB('a', false, false, true)); q('k'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bboom() {
        Test.startTest(); try { b('a'); System.enqueueJob(new StopOrderBoom('z')); q('k'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void bsch() {
        Test.startTest(); try { b('a'); System.schedule('stopMixJob', '0 0 0 1 1 ? 2099', new StopOrderSched()); q('k'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void ff2() {
        Test.startTest(); try { StopOrderMx.futFut('f', 'g'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void ff2q() {
        Test.startTest(); try { StopOrderMx.futFut('f', 'g'); q('k'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void qc2() {
        Test.startTest(); try { StopOrderMx.chain('m', 2, false); q('t'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void qc3() {
        Test.startTest(); try { StopOrderMx.chain('m', 3, false); q('t'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void qc2cap() {
        Test.startTest(); try { StopOrderMx.chain('m', 2, true); q('t'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void qc3cap() {
        Test.startTest(); try { StopOrderMx.chain('m', 3, true); q('t'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void fc2() {
        Test.startTest(); try { StopOrderMx.futChain('f', 'm', 2, false); q('t'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void fc3() {
        Test.startTest(); try { StopOrderMx.futChain('f', 'm', 3, false); q('t'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void fc2cap() {
        Test.startTest(); try { StopOrderMx.futChain('f', 'm', 2, true); q('t'); } catch (Exception e) { fail(e); } fin();
    }
    @IsTest static void fc3cap() {
        Test.startTest(); try { StopOrderMx.futChain('f', 'm', 3, true); q('t'); } catch (Exception e) { fail(e); } fin();
    }
}
