@IsTest
private class CarryX2Test {
    @IsTest static void alpha() {
        System.assertEquals(4, CarryShared.s4());
    }
    @IsTest static void three() {
        System.assertEquals(3, CarryShared.s3());
    }
}
