@IsTest private class StopShapeTest {
    private static void report() { Test.stopTest(); System.assert(false, StopOrderMk.observe(false)); }
    private static void q(String n) { System.enqueueJob(new StopOrderQ(n, false)); }
    private static void nb(String p) { Database.executeBatch(new StopOrderNB(p, false, false, false)); }
    @IsTest static void emptyBF() { Test.startTest(); Database.executeBatch(new StopBoundaryEmpty()); StopOrderMk.fut('f'); report(); }
    @IsTest static void emptyBQ() { Test.startTest(); Database.executeBatch(new StopBoundaryEmpty()); q('k'); report(); }
    @IsTest static void emptyBB() { Test.startTest(); Database.executeBatch(new StopBoundaryEmpty()); nb('c'); report(); }
    @IsTest static void multiBQ() { Test.startTest(); Database.executeBatch(new StopShapeMulti()); q('k'); report(); }
    @IsTest static void locBQ() { insert new List<Contact>{ new Contact(LastName = 'p1'), new Contact(LastName = 'p2') }; Test.startTest(); Database.executeBatch(new StopShapeLocator()); q('k'); report(); }
    @IsTest static void preBF() { nb('a'); Test.startTest(); StopOrderMk.fut('f'); q('k'); report(); }
    @IsTest static void preQF() { q('c'); StopOrderMk.fut('f'); Test.startTest(); q('k'); report(); }
    @IsTest static void sb() { Test.startTest(); System.schedule('stopShapeJob', '0 0 0 1 1 ? 2099', new StopOrderSched()); nb('a'); report(); }
}
