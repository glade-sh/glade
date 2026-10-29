@IsTest private class RejectFixtureTest {
    @IsTest static void exercise() {
        ReceiverFixture.start().accept(
            new ReceiverFixture.Value().missing()
        ).finish();
    }
}
