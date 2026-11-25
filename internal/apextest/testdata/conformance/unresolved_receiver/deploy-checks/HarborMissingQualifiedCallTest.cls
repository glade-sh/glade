@IsTest
private class HarborMissingQualifiedCallTest {
    @IsTest static void callsAbsentQualifiedClass() {
        HarborUninstalled.Depot.ping();
    }
}
