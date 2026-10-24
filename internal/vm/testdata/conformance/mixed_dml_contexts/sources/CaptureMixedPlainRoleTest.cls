@IsTest private class CaptureMixedPlainRoleTest {
    @IsTest static void observe() { String result=CaptureMixedDml.observe(true); System.assert(false, 'OBS|'+JSON.serialize(result)); }
}
