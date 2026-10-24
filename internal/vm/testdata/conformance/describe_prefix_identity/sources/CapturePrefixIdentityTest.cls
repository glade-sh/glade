@IsTest private class CapturePrefixIdentityTest {
    @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(CapturePrefixIdentity.observe())); }
}
