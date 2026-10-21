@IsTest private class QueueDataBoundaryProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(QueueDataBoundaryProbe.observe())); }
}
