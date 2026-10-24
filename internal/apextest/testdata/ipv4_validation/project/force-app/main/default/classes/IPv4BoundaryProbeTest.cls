@IsTest
private class IPv4BoundaryProbeTest {
    @IsTest static void validateAddresses() {
        IPv4BoundaryProbe.validateAddresses();
        System.runAs(new User(Id = UserInfo.getUserId())) {
            IPv4BoundaryProbe.validateAddresses();
        }
    }
}
