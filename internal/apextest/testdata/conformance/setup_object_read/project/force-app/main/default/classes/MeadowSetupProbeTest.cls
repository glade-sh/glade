@IsTest private class MeadowSetupProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(MeadowSetupProbe.observe())); }
}
