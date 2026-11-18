@IsTest
private class CvShapeCTest {
    @IsTest static void one() {
        CvShapeC c = new CvShapeC();
        System.assertEquals(3, CvShapeC.annotatedMulti(1, 2));
        System.assertEquals(0, CvShapeC.elseIfChain(0));
        System.assertEquals(2, CvShapeC.elseIfChain(20));
        System.assertEquals(7, CvShapeC.nullSafe(null));
        CvShapeC.assertLine(3);
        System.assertEquals('start-v-endxypq' + '', CvShapeC.concatLiteral('v'));
        System.assertEquals(0, CvShapeC.soqlLoop().size());
    }
    @IsTest static void two() {
        CvShapeC.dmlAndThrow(false);
        try {
            CvShapeC.dmlAndThrow(true);
        } catch (CvException e) {
            System.assertEquals('boom', e.getMessage());
        }
        System.assertEquals(2, CvShapeC.ternaryNested(7));
        System.assertEquals(true, CvShapeC.shortCircuit(false, true) || true);
        System.assertEquals(1, CvShapeC.instanceOfCheck('s'));
        CvShapeC.emptyMethod();
        CvShapeC.voidReturn();
        System.assertEquals(4, CvShapeC.withFinal(4));
        System.assertEquals(4, new CvShapeC.InnerC().get() + 1);
        CvShapeC c = new CvShapeC(5);
        c.prop = 9;
        System.assertEquals(1, c.prop);
    }
}
