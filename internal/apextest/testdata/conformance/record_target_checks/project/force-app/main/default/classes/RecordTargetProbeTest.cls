@IsTest private class RecordTargetProbeTest {
    @IsTest static void namedRoute() {
        System.assert(false, 'OBSERVED|namedRoute|' + JSON.serialize(RecordTargetProbe.observe()));
    }
}
