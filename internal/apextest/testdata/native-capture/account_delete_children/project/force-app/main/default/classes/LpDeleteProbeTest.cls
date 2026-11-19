@IsTest
private class LpDeleteProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'ROWS>' + String.join(LpDeleteProbe.run(), '\n') + '<ROWS');
    }
}
