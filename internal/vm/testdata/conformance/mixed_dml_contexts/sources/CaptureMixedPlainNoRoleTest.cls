@IsTest private class CaptureMixedPlainNoRoleTest {
    @IsTest static void observe() { String result=CaptureMixedDml.observe(false); System.assert(false, 'OBS|'+JSON.serialize(result)); }
}
