@IsTest
private class WillowLookupProbeTest {
    @IsTest static void observeLookup() {
        System.assert(false, 'WILLOW_RESULT|' + JSON.serialize(WillowLookupProbe.observe()));
    }
}
