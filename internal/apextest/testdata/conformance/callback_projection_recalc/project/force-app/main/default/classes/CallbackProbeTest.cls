@IsTest
private class CallbackProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + CallbackProbe.observe());
    }
}
