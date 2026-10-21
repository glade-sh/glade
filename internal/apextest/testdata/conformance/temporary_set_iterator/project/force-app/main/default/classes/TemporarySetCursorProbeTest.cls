@IsTest
private class TemporarySetCursorProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(TemporarySetCursorProbe.observe()));
    }
}
