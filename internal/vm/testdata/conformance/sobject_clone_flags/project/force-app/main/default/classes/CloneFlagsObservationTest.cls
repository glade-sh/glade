@IsTest private class CloneFlagsObservationTest {
    @IsTest static void capture() {
        System.debug('CLONE_ROWS:' + JSON.serialize(CloneFlagsObservation.observe()));
    }
}
