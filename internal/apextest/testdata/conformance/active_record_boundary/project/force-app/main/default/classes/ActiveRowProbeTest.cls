@IsTest private class ActiveRowProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(ActiveRowProbe.observe())); }
}
