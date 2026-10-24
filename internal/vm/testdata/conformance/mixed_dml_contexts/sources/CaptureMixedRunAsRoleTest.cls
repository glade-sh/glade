@IsTest private class CaptureMixedRunAsRoleTest {
    @IsTest static void observe() { String result; System.runAs(new User(Id=UserInfo.getUserId())) { result=CaptureMixedDml.observe(true); } System.assert(false, 'OBS|'+JSON.serialize(result)); }
}
