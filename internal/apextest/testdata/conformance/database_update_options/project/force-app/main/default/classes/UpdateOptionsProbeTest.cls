@IsTest
private class UpdateOptionsProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(UpdateOptionsProbe.observe()));
    }
}
