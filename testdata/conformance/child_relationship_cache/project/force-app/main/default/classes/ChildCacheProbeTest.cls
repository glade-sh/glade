@IsTest
private class ChildCacheProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'CHILD_CACHE|' + ChildCacheProbe.observe());
    }
}
