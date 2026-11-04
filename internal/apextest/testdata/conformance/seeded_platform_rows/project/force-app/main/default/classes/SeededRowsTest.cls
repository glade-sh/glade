@IsTest private class SeededRowsTest {
    @IsTest static void capture() {
        System.assert(false, 'OBSERVED|capture|' + JSON.serialize(SeededRowsProbe.observe()));
    }
}
