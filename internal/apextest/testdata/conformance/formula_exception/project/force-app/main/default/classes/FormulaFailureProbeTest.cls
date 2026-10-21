@IsTest
private class FormulaFailureProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + FormulaFailureProbe.sample()); }
}
