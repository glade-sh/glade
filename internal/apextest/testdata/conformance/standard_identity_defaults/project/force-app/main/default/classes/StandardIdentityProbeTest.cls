@IsTest
private class StandardIdentityProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + StandardIdentityProbe.sample()); }
}
