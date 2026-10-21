@IsTest
private class OrderProductProbeTest {
    @IsTest static void observe() { System.assert(false, 'OBS|' + OrderProductProbe.sample()); }
}
