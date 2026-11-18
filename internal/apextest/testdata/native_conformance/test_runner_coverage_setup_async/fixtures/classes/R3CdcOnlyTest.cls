@IsTest
private class R3CdcOnlyTest {
    @TestSetup static void seed() {
        Test.enableChangeDataCapture();
        insert new Account(Name = 'R3-CDCSRC');
    }
    @IsTest static void observe() {
        System.assert(false, R3Facts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, R3Facts.facts());
    }
}
