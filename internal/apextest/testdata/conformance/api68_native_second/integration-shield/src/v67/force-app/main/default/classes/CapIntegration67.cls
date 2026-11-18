@IntegrationTest
private class CapIntegration67 {
    static String marker;
    @BeforeClass static void setupOnce() {
        marker = 'set-only-in-setup';
        insert new Account(Name = 'S68-SHARED-FIXTURE');
    }
    @IntegrationTest static void first() {
        checkData();
    }
    @IntegrationTest static void second() {
        checkData();
    }
    private static void checkData() {
        Integer n = [SELECT COUNT() FROM Account WHERE Name = 'S68-SHARED-FIXTURE' WITH SYSTEM_MODE];
        System.debug(LoggingLevel.ERROR, 'COUNT=' + n + ';STATIC=' + marker);
    }
    @TearDown static void afterEach() {
        System.debug(LoggingLevel.ERROR, 'TEARDOWN');
    }
}
