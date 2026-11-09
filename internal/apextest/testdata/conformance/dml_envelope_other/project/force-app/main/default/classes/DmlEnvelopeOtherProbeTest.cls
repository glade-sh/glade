@IsTest private class DmlEnvelopeOtherProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(DmlEnvelopeOtherProbe.observe()));
    }
}
