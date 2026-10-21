trigger RuntimeTrace on Account (before insert) {
    if (RuntimeBoundaryProbe.traceEnabled) {
        RuntimeBoundaryProbe.trace = new RuntimeBoundaryProbe.ProbeException().getStackTraceString();
        RuntimeBoundaryProbe.helperTrace = RuntimeBoundaryProbe.capture();
    }
}
