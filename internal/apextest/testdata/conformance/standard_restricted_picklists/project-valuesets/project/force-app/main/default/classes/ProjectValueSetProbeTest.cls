@IsTest
private class ProjectValueSetProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(ProjectValueSetProbe.observe())); }
}
