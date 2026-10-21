@IsTest
private class StringFormatQuotesProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(StringFormatQuotesProbe.observe()));
    }
}
