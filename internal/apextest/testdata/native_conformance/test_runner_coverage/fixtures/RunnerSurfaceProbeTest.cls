@IsTest
private class RunnerSurfaceProbeTest {
    @TestSetup
    static void seed() {
        insert as system new Account(Name = 'RUNNER_SEED');
    }

    @IsTest
    static void passes() {
        System.assertEquals(1, [SELECT COUNT() FROM Account
                              WHERE Name = 'RUNNER_SEED' WITH SYSTEM_MODE]);
    }

    @IsTest
    static void failsWithText() {
        System.assertEquals(1, 2, 'CAP <&> "quote"\nsecond line');
    }

    @IsTest
    static void failsNested() {
        failHere();
    }

    private static void failHere() {
        System.assert(false, 'CAP_NESTED');
    }

    @IsTest
    static void handlesException() {
        try {
            Integer n = Integer.valueOf('not-an-integer');
            System.assert(false, 'unreachable');
        } catch (TypeException expected) {
            System.assertNotEquals(null, expected.getMessage());
        }
    }

    @IsTest
    static void measuresRuntime() {
        Integer started = Limits.getCpuTime();
        while (Limits.getCpuTime() - started < 100) {
            String sample = String.valueOf(Limits.getCpuTime());
        }
        System.assert(true);
    }
}
