@IsTest
private class CvShapeATest {
    @IsTest static void leftBranch() {
        System.assertEquals(6, CvShapeA.run(true));
    }
    @IsTest static void rightBranch() {
        System.assertEquals(0, CvShapeA.run(false));
    }
    @IsTest static void objects() {
        CvShapeA instance = new CvShapeA();
        instance.automatic = 3;
        System.assertEquals(8, instance.explicitGetter);
        System.assertEquals(3, instance.automatic);
        System.assertEquals(7, new CvShapeA.NestedProbe().value(5));
    }
    @IsTest static void partiallyCompletedLine() {
        try {
            CvShapeA.sameLine(true);
            System.assert(false, 'exception expected');
        } catch (CvShapeA.ShapeException expected) {
            System.assertEquals('shape', expected.getMessage());
        }
    }
    @IsTest static void debugSideEffect() {
        Integer before = CvShapeA.seed;
        CvShapeA.debugArgument();
        System.assertEquals(before + 1, CvShapeA.seed);
        System.assertEquals(2, CvShapeA.mixedDebugLine());
    }
}
