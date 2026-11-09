@IsTest private class DmlEnvelopeInsertProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(DmlEnvelopeInsertProbe.observe()));
    }
}
