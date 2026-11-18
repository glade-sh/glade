@IsTest
private class RunnerSetupFail {
    @TestSetup static void seed() { System.assert(false, 'CAP_SETUP'); }
    @IsTest static void one() { System.assert(true); }
    @IsTest static void two() { System.assert(true); }
}
