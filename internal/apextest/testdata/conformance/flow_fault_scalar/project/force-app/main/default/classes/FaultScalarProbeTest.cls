@IsTest
private class FaultScalarProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + FaultScalarProbe.sample()); }
}
