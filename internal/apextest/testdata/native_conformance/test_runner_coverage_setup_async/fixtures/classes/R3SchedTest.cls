@IsTest
private class R3SchedTest {
    @TestSetup static void seed() {
        System.schedule('r3-setup-sched', '0 0 0 1 1 ? 2099', new R3Sched());
    }
    @IsTest static void observe() {
        System.assert(false, R3Facts.facts());
    }
    @IsTest static void observeAgain() {
        System.assert(false, R3Facts.facts());
    }
}
