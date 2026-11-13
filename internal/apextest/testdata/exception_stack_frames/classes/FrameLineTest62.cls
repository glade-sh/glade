@IsTest
private class FrameLineTest62 {
    @IsTest static void chainSplit() {
        String obs; FrameTop62 t = new FrameTop62(null);
        try {
            obs = t
                .captureI('x');
        } catch (Exception e) { obs = e.getStackTraceString(); }
        System.assertEquals('x', obs);
    }

    @IsTest static void argSplit() {
        String obs;
        try {
            obs = FrameTop62.take(
                FrameLeaf62.boom());
        } catch (Exception e) { obs = e.getStackTraceString(); }
        System.assertEquals('x', obs);
    }

    @IsTest static void ctorSplit() {
        String obs;
        try { FrameTop62 t =
            new FrameTop62('x');
        } catch (Exception e) { obs = e.getStackTraceString(); }

        System.assertEquals('x', obs);
    }
}
