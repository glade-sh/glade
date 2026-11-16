@IsTest
private class RunnerSetupTwoPass {
    @TestSetup static void seed() { insert as system new Account(Name = 'RS'); }
    @IsTest static void one() { System.assert(true); }
    @IsTest static void two() { System.assert(true); }
}
