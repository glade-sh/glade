@IsTest(SeeAllData=true)
private class SadSetupClassTrue {
    @TestSetup static void seed() { }
    @IsTest static void t() { System.assert(true); }
}
