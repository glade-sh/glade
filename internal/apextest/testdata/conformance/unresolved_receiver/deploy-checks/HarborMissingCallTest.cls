@IsTest
private class HarborMissingCallTest {
    @IsTest static void callsAbsentClass() {
        HarborUnavailableDepot.ping();
    }
}
