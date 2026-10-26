@IsTest private class MeadowDirectProbeTest {
    @IsTest static void observe() { System.assert(false,'OBS|'+JSON.serialize(MeadowDirectProbe.observe())); }
}
