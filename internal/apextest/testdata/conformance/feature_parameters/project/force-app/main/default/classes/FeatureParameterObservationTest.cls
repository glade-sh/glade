@IsTest
private class FeatureParameterObservationTest {
    @IsTest
    static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(FeatureParameterObservation.read()));
    }
}
