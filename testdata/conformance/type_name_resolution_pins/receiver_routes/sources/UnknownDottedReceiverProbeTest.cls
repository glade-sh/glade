@IsTest private class UnknownDottedReceiverProbeTest {
    @IsTest static void observe() {
        a.b.c.x();
    }
}
