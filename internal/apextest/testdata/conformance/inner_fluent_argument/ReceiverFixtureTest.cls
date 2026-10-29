@IsTest private class ReceiverFixtureTest {
    @IsTest static void exercise() {
        System.assertEquals('ok', ReceiverFixture.start().accept(
            new ReceiverFixture.Value().read()
        ).finish());
    }
    @IsTest static void boundary() {
        System.assertEquals('value', new ReceiverFixture.Value().read());
    }
}
