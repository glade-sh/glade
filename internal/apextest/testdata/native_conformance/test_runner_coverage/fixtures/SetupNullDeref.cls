@IsTest
private class SetupNullDeref {
    @TestSetup static void seed() {
        Account a = null; String s = a.Name;
    }
    @IsTest static void first() { System.debug('BODY'); }
    @IsTest static void second() { System.assert(true); }
}
