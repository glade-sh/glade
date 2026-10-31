@IsTest private class RouteProbeTest {
 @IsTest static void observe() { System.assert(false, 'PROBE|' + JSON.serialize(RouteProbe.observe())); }
}
