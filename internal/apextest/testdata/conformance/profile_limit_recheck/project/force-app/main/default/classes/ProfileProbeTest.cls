@IsTest
private class ProfileProbeTest {
    @IsTest static void observe() { System.assert(false, 'CAPTURE|' + JSON.serialize(ProfileProbe.observe())); }
}
