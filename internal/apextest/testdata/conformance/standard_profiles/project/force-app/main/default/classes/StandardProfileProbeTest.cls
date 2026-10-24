@IsTest private class StandardProfileProbeTest {
    @IsTest static void profiles() { System.assert(false,'OBSERVED|profiles|'+JSON.serialize(StandardProfileProbe.profiles())); }
    @IsTest static void missing() { System.assert(false,'OBSERVED|missing|'+JSON.serialize(StandardProfileProbe.missing())); }
    @IsTest static void boundary() { System.assert(false,'OBSERVED|boundary|'+JSON.serialize(StandardProfileProbe.boundary())); }
}
