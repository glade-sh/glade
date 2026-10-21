@IsTest private class RecordDefaultListProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBSERVED|observe|' + JSON.serialize(RecordDefaultListProbe.observe()));
    }
}
