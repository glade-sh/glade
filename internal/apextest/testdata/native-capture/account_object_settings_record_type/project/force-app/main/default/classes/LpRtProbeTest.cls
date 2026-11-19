@IsTest
private class LpRtProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'ROWS>' + String.join(LpRtProbe.run(), '\n') + '<ROWS');
    }
}
