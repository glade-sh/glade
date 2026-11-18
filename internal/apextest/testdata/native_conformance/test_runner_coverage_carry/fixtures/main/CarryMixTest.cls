@IsTest
private class CarryMixTest {
    @IsTest static void Alpha() {
        System.assertEquals(1, CarryMix.a());
    }
    @IsTest static void apple() {
        System.assertEquals(2, CarryMix.b());
    }
    @IsTest static void beta() {
        System.assertEquals(3, CarryMix.c());
    }
    @IsTest static void Gamma() {
        System.assertEquals(4, CarryMix.d());
    }
    @IsTest static void Zeta() {
        CarryMix m = new CarryMix();
        System.assertEquals(0, m.prop);
    }
    @IsTest static void delta() {
        System.assertEquals(9, CarryOther.x());
    }
}
