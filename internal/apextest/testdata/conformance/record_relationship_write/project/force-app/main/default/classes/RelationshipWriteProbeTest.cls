@IsTest
private class RelationshipWriteProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(RelationshipWriteProbe.run()));
    }
}
