@IsTest
private class HarborCompileControlTest {
    private static void ping() { }

    @IsTest static void callsAvailableReceiver() {
        HarborCompileControlTest.ping();
    }
}
