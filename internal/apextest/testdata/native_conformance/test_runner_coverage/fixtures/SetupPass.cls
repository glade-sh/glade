@IsTest
private class SetupPass {
    @TestSetup static void seed() {
        insert as system new Account(Name = 'ok');
    }
    @IsTest static void first() { System.debug('BODY'); }
    @IsTest static void second() { System.assert(true); }
}
