@IsTest private class DelimiterBoundaryProbeTest {
 @IsTest static void observe() { System.assertEquals('capture',JSON.serialize(DelimiterBoundaryProbe.observe()),'DELIMITER_ROWS'); }
}
