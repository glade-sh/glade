@IsTest private class OwnedScopeObservationTest {
    @IsTest static void capture() {
        System.assert(false,'OWNED_SCOPE:'+JSON.serialize(OwnedScopeObservation.observe(true)));
    }
}
