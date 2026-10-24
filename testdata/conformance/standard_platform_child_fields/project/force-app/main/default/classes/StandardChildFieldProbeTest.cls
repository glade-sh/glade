@IsTest private class StandardChildFieldProbeTest {
    @IsTest static void walk() { System.assert(false, 'STANDARD_CHILD_ROWS|' + JSON.serialize(StandardChildFieldProbe.rows())); }
}
