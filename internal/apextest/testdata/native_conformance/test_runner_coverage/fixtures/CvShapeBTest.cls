@IsTest
private class CvShapeBTest {
    @IsTest static void loopsAndSwitch() {
        System.assertEquals(9, CvShapeB.loops(3));
        System.assertEquals('one', CvShapeB.sw(1));
        System.assertEquals('many', CvShapeB.sw(9));
        System.assertEquals(2, CvShapeB.tern(false));
    }
    @IsTest static void guards() {
        System.assertEquals(6, CvShapeB.guarded(2));
        System.assertEquals(5, CvShapeB.listing() + 0);
        System.assertEquals(0, CvShapeB.query());
        System.assertEquals(1, CvShapeB.oneLiner());
    }
    @IsTest static void objects() {
        CvShapeB b = new CvShapeB();
        System.assertEquals(4, b.instanceCalled());
        System.assertEquals(9, new CvShapeB.Sq(3).area());
        System.assertEquals(4, CvShapeB.annotated(3));
    }
    @IsTest static void constantOnly() {
        System.assertEquals(12, CvShapeB.constOnly());
    }
}
