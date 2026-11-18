@IsTest
private class R3CdcTest {
    @TestSetup static void seed() {
        Test.enableChangeDataCapture();
        insert new Account(Name = 'R3-CDCSRC');
        System.enqueueJob(new R3Q());
    }
    @IsTest static void observe() {
        System.assert(false, R3Facts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, R3Facts.facts());
    }
}
