@isTest private class PclMsgGuestCaseTest {
    @isTest static void run() {
        User u = PclMsgSupport.guest();
        String m;
        System.runAs(u) { m = PclMsgSupport.q('SELECT count() FROM Case'); }
        System.assert(false, m);
    }
}
