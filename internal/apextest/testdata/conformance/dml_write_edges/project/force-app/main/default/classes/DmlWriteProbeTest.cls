@IsTest
private class DmlWriteProbeTest {
    @IsTest static void lookupRoute() {
        System.assert(false, 'R563' + JSON.serialize(DmlWriteProbe.lookupRows()));
    }

    @IsTest static void upsertRoute() {
        System.assert(false, 'R563' + JSON.serialize(DmlWriteProbe.upsertRows()));
    }

    @IsTest static void textRoute() {
        System.assert(false, 'R563' + JSON.serialize(DmlWriteProbe.textRows()));
    }
}
