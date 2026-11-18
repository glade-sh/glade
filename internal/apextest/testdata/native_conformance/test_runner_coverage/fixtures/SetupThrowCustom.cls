@IsTest
private class SetupThrowCustom {
    @TestSetup static void seed() {
        throw new CvException('SETUP_CUSTOM');
    }
    @IsTest static void first() { System.debug('BODY'); }
    @IsTest static void second() { System.assert(true); }
}
