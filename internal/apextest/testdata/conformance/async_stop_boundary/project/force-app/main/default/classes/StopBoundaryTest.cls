@IsTest private class StopBoundaryTest {
    private static void report() { Test.stopTest(); System.assert(false, StopOrderMk.observe(false)); }
    @IsTest static void pre() { Database.executeBatch(new StopOrderNB('a', false, false, false)); Test.startTest(); System.enqueueJob(new StopOrderQ('k', false)); report(); }
    @IsTest static void stateful() { Test.startTest(); Database.executeBatch(new StopBoundaryIterable()); System.enqueueJob(new StopOrderQ('k', false)); report(); }
    @IsTest static void locator() { insert new Contact(LastName='probe'); Test.startTest(); Database.executeBatch(new StopBoundaryBatch()); System.enqueueJob(new StopOrderQ('k', false)); report(); }
}
