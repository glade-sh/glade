@IsTest private class DependencyProbeTest {
    @IsTest static void loaded() {
        Flow.Interview.createInterview('DependencyProbe', new Map<String,Object>()).start();
        System.assertEquals(1, DependencyAction.calls);
    }
}
