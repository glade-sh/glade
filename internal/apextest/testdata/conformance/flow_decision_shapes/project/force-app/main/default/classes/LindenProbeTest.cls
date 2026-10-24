@IsTest
private class LindenProbeTest {
    @IsTest static void t1() { LindenProbe.observe(1); }
    @IsTest static void t2() { LindenProbe.observe(2); }
    @IsTest static void t3() { LindenProbe.observe(3); }
}
