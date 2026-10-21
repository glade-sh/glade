@IsTest private class HandledMessageProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(HandledMessageProbe.observe())); }
}
