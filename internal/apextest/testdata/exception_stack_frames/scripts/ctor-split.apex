String obs;
try {
    FrameTop{V} t =
        new FrameTop{V}('x');
} catch (Exception e) { obs = e.getStackTraceString(); }
