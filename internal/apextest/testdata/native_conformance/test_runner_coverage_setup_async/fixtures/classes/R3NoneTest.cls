@IsTest
private class R3NoneTest {
    @TestSetup static void seed() {
        insert new Account(Name = 'R3-SYNC');
    }
    @IsTest static void observe() {
        System.assert(false, R3Facts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, R3Facts.facts());
    }
}
