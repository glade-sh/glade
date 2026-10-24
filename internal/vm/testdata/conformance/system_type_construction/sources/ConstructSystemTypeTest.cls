@IsTest private class ConstructSystemTypeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(ConstructSystemType.observe())); }
}
