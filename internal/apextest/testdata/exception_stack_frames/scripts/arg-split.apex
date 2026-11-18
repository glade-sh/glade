String obs;

try {
    obs = FrameTop{V}.take(
        FrameLeaf{V}.boom());
} catch (Exception e) { obs = e.getStackTraceString(); }
