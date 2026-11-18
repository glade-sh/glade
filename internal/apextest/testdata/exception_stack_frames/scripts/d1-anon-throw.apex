String obs;
try {
    FrameLeaf{V}.throwCustom();
} catch (Exception e) { obs = e.getStackTraceString(); }
