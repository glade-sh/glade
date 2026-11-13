String obs;
FrameTop{V} t = new FrameTop{V}(null);
try {
    obs = t
        .captureI('x');
} catch (Exception e) { obs = e.getStackTraceString(); }
