@IsTest
private class SetupFailureProbe {
    @TestSetup static void seed() { System.assert(false, 'SETUP_MARKER'); }
    @IsTest static void first() { System.debug('BODY_FIRST'); }
    @IsTest static void second() { System.debug('BODY_SECOND'); }
}
