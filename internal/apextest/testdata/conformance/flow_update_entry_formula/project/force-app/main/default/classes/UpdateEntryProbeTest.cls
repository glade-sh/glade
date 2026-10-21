@IsTest
private class UpdateEntryProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + UpdateEntryProbe.sample()); }
}
