@IsTest
private class RunnerCompileProbeTest {
    @IsTest static void readsDependency() {
        System.assertEquals(1, RunnerDependency.value());
    }
}
