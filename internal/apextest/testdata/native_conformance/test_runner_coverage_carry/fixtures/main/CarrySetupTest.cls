@IsTest
private class CarrySetupTest {
    @TestSetup static void seed() {
        CarryS.onlySetup();
    }
    @IsTest static void first() {
        System.assertEquals(2, CarryS.other());
    }
    @IsTest static void second() {
        System.assert(true);
    }
}
