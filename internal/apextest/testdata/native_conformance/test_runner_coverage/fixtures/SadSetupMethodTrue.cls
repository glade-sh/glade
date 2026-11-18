@IsTest
private class SadSetupMethodTrue {
    @TestSetup static void seed() { }
    @IsTest(SeeAllData=true) static void t() { System.assert(true); }
}
