@IsTest
private class JuniperUpsertProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(JuniperUpsertProbe.observe()));
    }
}
