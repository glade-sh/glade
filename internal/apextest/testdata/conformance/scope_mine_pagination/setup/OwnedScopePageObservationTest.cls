@IsTest private class OwnedScopePageObservationTest {
    @IsTest static void capture() {
        System.assert(false,'OWNED_SCOPE_PAGE:'+JSON.serialize(OwnedScopePageObservation.observe(true)));
    }
}
