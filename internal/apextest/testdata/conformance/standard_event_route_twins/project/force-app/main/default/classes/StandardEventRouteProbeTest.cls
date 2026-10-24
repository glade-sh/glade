@IsTest private class StandardEventRouteProbeTest {
 @IsTest static void observe(){ System.assert(false,'OBS|'+JSON.serialize(StandardEventRouteProbe.observe())); }
}
