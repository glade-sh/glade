@IsTest
private class BrProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + BrProbe.sample()); }
}
