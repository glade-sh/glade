@IsTest private class RuntimeBoundaryTest {
    @IsTest static void triggerFrames() { RuntimeBoundaryProbe.observeTrigger(); System.assert(false, RuntimeBoundaryProbe.lastRow); }
    @IsTest static void nullAssignment() { RuntimeBoundaryProbe.observeNull(); System.assert(false, RuntimeBoundaryProbe.lastRow); }
    @IsTest static void flowDefaults() { RuntimeBoundaryProbe.observeFlow(); System.assert(false, RuntimeBoundaryProbe.lastRow); }
    @IsTest static void profileLicense() { RuntimeBoundaryProbe.observeProfile(); System.assert(false, RuntimeBoundaryProbe.lastRow); }
}
