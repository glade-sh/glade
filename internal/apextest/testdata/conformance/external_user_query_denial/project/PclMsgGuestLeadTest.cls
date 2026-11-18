@isTest private class PclMsgGuestLeadTest {
    @isTest static void run() {
        User u = PclMsgSupport.guest();
        String m;
        System.runAs(u) { m = PclMsgSupport.q('SELECT count() FROM Lead'); }
        System.assert(false, m);
    }
}
