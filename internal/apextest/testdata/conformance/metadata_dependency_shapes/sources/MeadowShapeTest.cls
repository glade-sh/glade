@IsTest
private class MeadowShapeTest {
    @IsTest static void split() { System.assertEquals('oak:pine:0', MeadowShapeSplit.observe()); }
    @IsTest static void upperA() { System.assertEquals('CA:0', MeadowShapeUpperA.observe()); }
    @IsTest static void upperB() { System.assertEquals('CA:0', MeadowShapeUpperB.observe()); }
    @IsTest static void lower() { System.assertEquals('grove:0', MeadowShapeLower.observe()); }
    @IsTest static void event() { System.assertEquals('true', MeadowShapeEvent.observe()); }
}
