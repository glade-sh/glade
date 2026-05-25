@IsTest
private class TrailConsumerServiceTest {
    @IsTest
    static void returnsDependencyValue() {
        System.assertEquals('managed', TrailConsumerService.value());
    }
}
