@IsTest
private class ConsumerServiceTest {
    @IsTest
    static void returnsDependencyValue() {
        System.assertEquals('managed', ConsumerService.value());
    }
}
