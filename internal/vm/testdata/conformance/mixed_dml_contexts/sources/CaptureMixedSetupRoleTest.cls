@IsTest private class CaptureMixedSetupRoleTest {
    @TestSetup static void seed() { String result=CaptureMixedDml.observe(true); System.assert(false, 'OBS|'+JSON.serialize(result)); }
    @IsTest static void observe() { System.assert(true); }
}
