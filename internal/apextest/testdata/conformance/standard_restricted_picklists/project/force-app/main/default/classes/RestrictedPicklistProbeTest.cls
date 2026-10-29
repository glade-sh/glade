@IsTest private class RestrictedPicklistProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RestrictedPicklistProbe.observe())); }
}
