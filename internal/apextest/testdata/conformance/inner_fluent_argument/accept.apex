System.assertEquals('ok', ReceiverFixture.start().accept(
    new ReceiverFixture.Value().read()
).finish());
