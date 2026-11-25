@IsTest
private class HarborInvalidTypeDependentTest {
    @IsTest static void callsInvalidRoot() {
        HarborInvalidTypeRoot.ping();
    }
}
