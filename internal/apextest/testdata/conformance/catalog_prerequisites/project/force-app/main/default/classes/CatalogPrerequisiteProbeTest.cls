@IsTest
private class CatalogPrerequisiteProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + CatalogPrerequisiteProbe.sample()); }
}
