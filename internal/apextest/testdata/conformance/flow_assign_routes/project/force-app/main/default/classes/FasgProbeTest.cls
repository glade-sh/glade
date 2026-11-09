@isTest
private class FasgProbeTest {
    @isTest
    static void observe() {
        System.assert(false, 'OBS|' + FasgProbe.sample());
    }
}
