@IsTest
private class LFActivityProbeTest {
    @IsTest
    static void probe() {
        System.assert(false, 'ROWS>' + String.join(LFActivityProbe.run(), '\n') + '\n' + String.join(LFActivityProbe2.run(), '\n') + '<ROWS');
    }
}
