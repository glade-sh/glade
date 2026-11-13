@IsTest
private class FrameLineTest67 {
    @IsTest static void chainSplit() {
        String obs; FrameTop67 t = new FrameTop67(null);
        try {
            obs = t
                .captureI('x');
        } catch (Exception e) { obs = e.getStackTraceString(); }
        System.assertEquals('x', obs);
    }

    @IsTest static void argSplit() {
        String obs;
        try {
            obs = FrameTop67.take(
                FrameLeaf67.boom());
        } catch (Exception e) { obs = e.getStackTraceString(); }
        System.assertEquals('x', obs);
    }

    @IsTest static void ctorSplit() {
        String obs;
        try { FrameTop67 t =
            new FrameTop67('x');
        } catch (Exception e) { obs = e.getStackTraceString(); }

        System.assertEquals('x', obs);
    }
}
