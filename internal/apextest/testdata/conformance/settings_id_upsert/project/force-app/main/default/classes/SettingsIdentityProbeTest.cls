@IsTest
private class SettingsIdentityProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + SettingsIdentityProbe.sample()); }
}
