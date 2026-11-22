@IsTest private class RoleFirstTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RoleBoundaryProbe.observe(true))); }
}
