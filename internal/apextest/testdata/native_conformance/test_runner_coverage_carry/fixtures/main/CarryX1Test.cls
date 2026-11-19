@IsTest
private class CarryX1Test {
    @IsTest static void one() {
        System.assertEquals(1, CarryShared.s1());
    }
    @IsTest static void two() {
        System.assertEquals(2, CarryShared.s2());
    }
}
