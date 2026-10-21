@IsTest private class DeletedAccessProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(DeletedAccessProbe.observe()));
    }
}
