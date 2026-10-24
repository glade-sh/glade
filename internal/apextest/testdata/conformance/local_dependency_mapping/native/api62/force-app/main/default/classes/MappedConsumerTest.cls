@IsTest
private class MappedConsumerTest {
    @IsTest
    static void invokesUnnamespacedLibrary() {
        System.assertEquals('local-dependency', MappedConsumer.value());
    }
}
