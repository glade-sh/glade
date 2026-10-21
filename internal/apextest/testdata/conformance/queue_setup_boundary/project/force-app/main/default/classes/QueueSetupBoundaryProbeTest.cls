@IsTest private class QueueSetupBoundaryProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(QueueSetupBoundaryProbe.observe())); }
}
