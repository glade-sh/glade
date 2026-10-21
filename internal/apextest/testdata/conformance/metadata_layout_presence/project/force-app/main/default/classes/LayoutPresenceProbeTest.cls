@IsTest
private class LayoutPresenceProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(LayoutPresenceProbe.observe()));
    }
}
