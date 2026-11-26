@IsTest
private class HarborInvalidCallDependentTest {
    @IsTest static void callsInvalidRoot() {
        HarborInvalidCallRoot.ping();
    }
}
