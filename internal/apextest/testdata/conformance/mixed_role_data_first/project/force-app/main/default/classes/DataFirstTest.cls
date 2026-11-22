@IsTest private class DataFirstTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + JSON.serialize(RoleBoundaryProbe.observe(false))); }
}
