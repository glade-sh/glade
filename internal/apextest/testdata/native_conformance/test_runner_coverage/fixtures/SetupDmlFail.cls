@IsTest
private class SetupDmlFail {
    @TestSetup static void seed() {
        insert new Contact();
    }
    @IsTest static void first() { System.debug('BODY'); }
    @IsTest static void second() { System.assert(true); }
}
